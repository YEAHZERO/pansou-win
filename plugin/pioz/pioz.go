package pioz

import (
	"bytes"
	"compress/flate"
	"compress/gzip"
	"compress/zlib"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"pansou/model"
	"pansou/plugin"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/PuerkitoBio/goquery"
)

// ============================================================================
// Pioz Plugin
//
// 目标：从 pioz.cn 搜索结果页/API 获取资源详情，并尽可能提取最终网盘分享链接（优先夸克）。
// 关键点：
// - UniqueID 编码：用于在“结果页 -> 详情页/transfer API”之间传递资源 ID 与详情页 URL。
// - 反爬：请求节流 + UA 轮换 + Cookie 维持；对 403/429 以及特征内容做拦截。
// - 解压：支持 gzip/deflate（zlib/flate 两种 deflate 变体）。
//
// 说明：为避免控制台编码差异导致乱码，本文件字符串尽量保持 ASCII；需要中文的地方使用 \\uXXXX。
// ============================================================================

const (

	// 超时配置
	DefaultTimeout = 15 * time.Second
	DetailTimeout  = 12 * time.Second

	// 站点配置
	APIBaseURL     = "https://www.pioz.cn/api"
	SiteBaseURL    = "https://www.pioz.cn"
	

	// 并发控制：详情页增强最多同时跑多少个请求
	MaxConcurrency = 8
	

	// HTTP 连接池
	MaxIdleConns        = 50
	MaxIdleConnsPerHost = 10
	MaxConnsPerHost     = 20
	IdleConnTimeout     = 60 * time.Second
	

	// 搜索/详情缓存 TTL
	CacheTTL = 30 * time.Minute
	

	// 反爬策略：请求间隔 + 重试
	RequestDelayMin = 500 * time.Millisecond
	RequestDelayMax = 1500 * time.Millisecond
	RetryCount      = 2

	// 详情增强最多处理多少条结果，避免对目标站造成过大压力
	MaxEnhancedResults = 10
)

func init() {
	plugin.RegisterGlobalPlugin(NewPiozPlugin())
}


var (

	// 网盘链接匹配（支持多种类型）
	quarkLinkRegex      = regexp.MustCompile(`https?://pan\.quark\.cn/s/[0-9a-zA-Z]{12,}`)
	baiduLinkRegex      = regexp.MustCompile(`https?://pan\.baidu\.com/s/[0-9a-zA-Z_\-]+(?:\?pwd=[0-9a-zA-Z]+)?`)
	aliyunLinkRegex     = regexp.MustCompile(`https?://(?:www\.)?(?:aliyundrive\.com|alipan\.com)/s/[0-9a-zA-Z]+`)
	ucLinkRegex         = regexp.MustCompile(`https?://drive\.uc\.cn/s/[0-9a-zA-Z]+(?:\?[^"'\s]*)?`)
	xunleiLinkRegex     = regexp.MustCompile(`https?://pan\.xunlei\.com/s/[0-9a-zA-Z_\-]+(?:\?pwd=[0-9a-zA-Z]+)?`)
	tianyiLinkRegex     = regexp.MustCompile(`https?://cloud\.189\.cn/t/[0-9a-zA-Z]+`)
	lanzouLinkRegex     = regexp.MustCompile(`https?://(?:www\.)?(?:lanzou[uixys]*|lan[zs]o[ux])\.(?:com|net|org)/[0-9a-zA-Z]+`)
	link115Regex        = regexp.MustCompile(`https?://115\.com/s/[0-9a-zA-Z]+`)
	mobileLinkRegex     = regexp.MustCompile(`https?://caiyun\.feixin\.10086\.cn/[0-9a-zA-Z]+`)
	weiyunLinkRegex     = regexp.MustCompile(`https?://share\.weiyun\.com/[0-9a-zA-Z]+`)
	jianguoyunLinkRegex = regexp.MustCompile(`https?://(?:www\.)?jianguoyun\.com/p/[0-9a-zA-Z]+`)
	link123Regex        = regexp.MustCompile(`https?://123pan\.com/s/[0-9a-zA-Z]+`)
	pikpakLinkRegex     = regexp.MustCompile(`https?://mypikpak\.com/s/[0-9a-zA-Z]+`)
	magnetLinkRegex     = regexp.MustCompile(`magnet:\?xt=urn:btih:[0-9a-fA-F]{40}`)
	ed2kLinkRegex       = regexp.MustCompile(`ed2k://\|file\|.+\|\d+\|[0-9a-fA-F]{32}\|/`)
	

	// 提取码/密码匹配：支持“提取码/密码/pwd/码: XXXX”
	passwordRegex    = regexp.MustCompile("(?i)(?:\\u63d0\\u53d6\\u7801|\\u5bc6\\u7801|pwd|\\u7801)[\\uff1a:]\\s*([a-zA-Z0-9]{4})")
	urlPasswordRegex = regexp.MustCompile(`(?i)\?pwd=([0-9a-zA-Z]+)`)
	

	// 详情页 ID 提取：/detail/{id}
	detailIDRegex = regexp.MustCompile(`/detail/(\d+)`)
	

	// 反爬关键词：页面/响应体出现这些字样时判定为反爬拦截
	antiCrawlerRegex = regexp.MustCompile("\\u7981\\u6b62\\u4f7f\\u7528\\u5f00\\u53d1\\u8005\\u5de5\\u5177|\\u5077\\u6837\\u5f0f\\u6b7b\\u5168\\u5bb6|\\u53cd\\u722c\\u866b|\\u9632\\u722c")
)


// =========================
// API 响应结构
// =========================

type DeepSearchResponse struct {
	Code    int `json:"code"`
	Results []struct {
		ID         int    `json:"id"`
		Title      string `json:"title"`
		CloudType  string `json:"cloud_type"`
		Datetime   string `json:"datetime"`
		Size       string `json:"size"`
		Desc       string `json:"desc"`
		CreateTime string `json:"create_time"`
		ViewURL    string `json:"view_url"`
	} `json:"results"`
	Total   int    `json:"total"`
	Message string `json:"message"`
}

type TransferResponse struct {
	Success bool `json:"success"`
	Data    struct {
		URL      string `json:"url"`
		Password string `json:"password"`
		Type     string `json:"type"`
	} `json:"data"`
	Error string `json:"error"`
}


// =========================
// 缓存与指标
// =========================

var (
	// 搜索缓存：keyword -> results
	searchCache   = sync.Map{}
	// 详情缓存：UniqueID -> SearchResult(with Links)
	detailCache   = sync.Map{}
	// transfer 缓存：resourceID -> []Link
	transferCache = sync.Map{}

	sessionCookies []*http.Cookie
	sessionMutex    sync.RWMutex
	lastRequestTime time.Time
	requestCounter  int64
)


var (
	searchRequests     int64 = 0
	detailRequests     int64 = 0
	cacheHits          int64 = 0
	cacheMisses        int64 = 0
	antiCrawlerBlocks  int64 = 0
	totalSearchTime    int64 = 0
	totalDetailTime    int64 = 0
)


// =========================
// 插件定义
// =========================

type PiozAsyncPlugin struct {
	*plugin.BaseAsyncPlugin
	optimizedClient    *http.Client
	userAgents         []string
	currentUserAgent   string
}


// createOptimizedHTTPClient 创建带连接池的 HTTP Client，并禁止自动跟随 30x（由调用者自行处理）。
func createOptimizedHTTPClient() *http.Client {
	transport := &http.Transport{
		MaxIdleConns:        MaxIdleConns,
		MaxIdleConnsPerHost: MaxIdleConnsPerHost,
		MaxConnsPerHost:     MaxConnsPerHost,
		IdleConnTimeout:     IdleConnTimeout,
		DisableKeepAlives:   false,
	}
	
	return &http.Client{
		Transport: transport,
		Timeout:   DefaultTimeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}


// NewPiozPlugin 构建插件实例（内置 UA 池用于反爬）。
func NewPiozPlugin() *PiozAsyncPlugin {
	userAgents := []string{
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/119.0.0.0 Safari/537.36",
		"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:109.0) Gecko/20100101 Firefox/120.0",
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36 Edg/120.0.0.0",
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/118.0.0.0 Safari/537.36",
		"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.0 Safari/605.1.15",
	}
	

	randomIndex := time.Now().UnixNano() % int64(len(userAgents))
	
	return &PiozAsyncPlugin{
		BaseAsyncPlugin:  plugin.NewBaseAsyncPlugin("pioz", 1),
		optimizedClient:  createOptimizedHTTPClient(),
		userAgents:       userAgents,
		currentUserAgent: userAgents[randomIndex],
	}
}


// Search 兼容旧接口：只返回结果数组。
func (p *PiozAsyncPlugin) Search(keyword string, ext map[string]interface{}) ([]model.SearchResult, error) {
	result, err := p.SearchWithResult(keyword, ext)
	if err != nil {
		return nil, err
	}
	return result.Results, nil
}


// SearchWithResult 走框架异步搜索入口，并由 searchImpl 完成实际抓取/解析。
func (p *PiozAsyncPlugin) SearchWithResult(keyword string, ext map[string]interface{}) (model.PluginSearchResult, error) {
	return p.AsyncSearchWithResult(keyword, p.searchImpl, p.MainCacheKey, ext)
}


// searchImpl 核心搜索逻辑：
// 1) 缓存命中直接返回
// 2) deep-search API（优先）
// 3) HTML 搜索页（备用）
// 4) 首页热搜（兜底）
// 5) 对结果做详情增强（提取最终网盘分享链接）
func (p *PiozAsyncPlugin) searchImpl(client *http.Client, keyword string, ext map[string]interface{}) ([]model.SearchResult, error) {

	start := time.Now()
	atomic.AddInt64(&searchRequests, 1)
	defer func() {
		duration := time.Since(start).Nanoseconds()
		atomic.AddInt64(&totalSearchTime, duration)
	}()


	cacheKey := fmt.Sprintf("%s:%s", p.Name(), keyword)
	if cached, ok := searchCache.Load(cacheKey); ok {
		if cachedResp, ok := cached.(cachedResponse); ok {
			if time.Since(cachedResp.timestamp) < CacheTTL {
				atomic.AddInt64(&cacheHits, 1)
				return cachedResp.results, nil
			}
		}
	}
	atomic.AddInt64(&cacheMisses, 1)


	if p.optimizedClient != nil {
		client = p.optimizedClient
	}


	p.applyAntiCrawlerDelay()


	results, err := p.performDeepSearch(client, keyword)
	if err == nil && len(results) > 0 {
		enhancedResults := p.enhanceWithDetails(client, results)
		searchCache.Store(cacheKey, cachedResponse{
			results:   enhancedResults,
			timestamp: time.Now(),
		})
		return plugin.FilterResultsByKeyword(enhancedResults, keyword), nil
	}


	p.applyAntiCrawlerDelay()
	results, err = p.performRegularSearch(client, keyword)
	if err == nil && len(results) > 0 {
		enhancedResults := p.enhanceWithDetails(client, results)
		searchCache.Store(cacheKey, cachedResponse{
			results:   enhancedResults,
			timestamp: time.Now(),
		})
		return plugin.FilterResultsByKeyword(enhancedResults, keyword), nil
	}


	p.applyAntiCrawlerDelay()
	results, err = p.extractFromHotSearch(client, keyword)
	if err != nil {
		return nil, fmt.Errorf("[%s] \u6240\u6709\u641c\u7d22\u7b56\u7565\u90fd\u5931\u8d25: %w", p.Name(), err)
	}

	enhancedResults := p.enhanceWithDetails(client, results)
	searchCache.Store(cacheKey, cachedResponse{
		results:   enhancedResults,
		timestamp: time.Now(),
	})
	
	return plugin.FilterResultsByKeyword(enhancedResults, keyword), nil
}


// =========================
// 搜索策略（API / HTML / 热搜）
// =========================

// performDeepSearch 调用 pioz deep-search API。
func (p *PiozAsyncPlugin) performDeepSearch(client *http.Client, keyword string) ([]model.SearchResult, error) {
	apiURL := fmt.Sprintf("%s/deep-search?kw=%s", APIBaseURL, url.QueryEscape(keyword))
	
	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return nil, err
	}
	
	p.setAPIHeaders(req)
	p.addSessionCookies(req)
	

	resp, err := p.doRequestWithRetry(client, req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	

	if p.checkAntiCrawlerResponse(resp) {
		atomic.AddInt64(&antiCrawlerBlocks, 1)
		return nil, fmt.Errorf("\u89e6\u53d1\u53cd\u722c\u4fdd\u62a4")
	}
	
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("API\u8fd4\u56de\u72b6\u6001\u7801: %d", resp.StatusCode)
	}
	
	body, err := p.readCompressedBody(resp)
	if err != nil {
		return nil, err
	}
	

	if antiCrawlerRegex.Match(body) {
		atomic.AddInt64(&antiCrawlerBlocks, 1)
		return nil, fmt.Errorf("\u54cd\u5e94\u5305\u542b\u53cd\u722c\u5185\u5bb9")
	}
	
	var apiResp DeepSearchResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return nil, fmt.Errorf("\u89e3\u6790API\u54cd\u5e94\u5931\u8d25: %w", err)
	}
	
	if apiResp.Code != 0 {
		return nil, fmt.Errorf("API\u9519\u8bef: %s", apiResp.Message)
	}
	
	if len(apiResp.Results) == 0 {
		return nil, fmt.Errorf("\u672a\u627e\u5230\u641c\u7d22\u7ed3\u679c")
	}
	

	var results []model.SearchResult
	for _, item := range apiResp.Results {
		result := p.convertAPIResultToSearchResult(item)
		results = append(results, result)
	}
	
	fmt.Printf("[%s] \u6df1\u5ea6\u641c\u7d22\u627e\u5230 %d \u4e2a\u7ed3\u679c\n", p.Name(), len(results))
	return results, nil
}


// performRegularSearch 解析 HTML 搜索页（API 失败时回退）。
func (p *PiozAsyncPlugin) performRegularSearch(client *http.Client, keyword string) ([]model.SearchResult, error) {
	searchURL := fmt.Sprintf("%s/search?q=%s", SiteBaseURL, url.QueryEscape(keyword))
	
	req, err := http.NewRequest("GET", searchURL, nil)
	if err != nil {
		return nil, err
	}
	
	p.setStealthHeaders(req)
	p.addSessionCookies(req)
	
	resp, err := p.doRequestWithRetry(client, req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	

	if p.checkAntiCrawlerResponse(resp) {
		atomic.AddInt64(&antiCrawlerBlocks, 1)
		return nil, fmt.Errorf("\u89e6\u53d1\u53cd\u722c\u4fdd\u62a4")
	}
	
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("\u641c\u7d22\u9875\u9762\u8fd4\u56de\u72b6\u6001\u7801: %d", resp.StatusCode)
	}
	
	body, err := p.readCompressedBody(resp)
	if err != nil {
		return nil, err
	}

	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	

	var results []model.SearchResult
	

	doc.Find(".file-item, .result-item, .search-item, .text-gray-100").Each(func(i int, s *goquery.Selection) {
		result := p.parseSearchItem(s, keyword, i)
		if result.UniqueID != "" {
			results = append(results, result)
		}
	})
	

	if len(results) == 0 {
		doc.Find("a[href*='/detail/']").Each(func(i int, a *goquery.Selection) {
			result := p.parseDetailLink(a, i)
			if result.UniqueID != "" {
				results = append(results, result)
			}
		})
	}
	
	if len(results) == 0 {
		return nil, fmt.Errorf("\u672a\u627e\u5230\u641c\u7d22\u7ed3\u679c")
	}
	
	fmt.Printf("[%s] \u666e\u901a\u641c\u7d22\u627e\u5230 %d \u4e2a\u7ed3\u679c\n", p.Name(), len(results))
	return results, nil
}


// extractFromHotSearch 从首页热搜区域提取匹配关键词的结果（兜底）。
func (p *PiozAsyncPlugin) extractFromHotSearch(client *http.Client, keyword string) ([]model.SearchResult, error) {
	req, err := http.NewRequest("GET", SiteBaseURL, nil)
	if err != nil {
		return nil, err
	}
	
	p.setStealthHeaders(req)
	p.addSessionCookies(req)
	
	resp, err := p.doRequestWithRetry(client, req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("\u9996\u9875\u8bbf\u95ee\u5931\u8d25: %d", resp.StatusCode)
	}
	
	body, err := p.readCompressedBody(resp)
	if err != nil {
		return nil, err
	}

	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	
	var results []model.SearchResult
	keywordLower := strings.ToLower(keyword)
	

	doc.Find(".hot-search-item").Each(func(i int, s *goquery.Selection) {

		if s.Find(".pinned").Length() > 0 {
			return
		}
		
		titleElem := s.Find(".hot-search-title-text")
		title := strings.TrimSpace(titleElem.Text())
		if title == "" {
			return
		}
		

		if !strings.Contains(strings.ToLower(title), keywordLower) {
			return
		}
		

		href, exists := s.Attr("href")
		if !exists {
			return
		}
		

		detailID := ""
		if strings.HasPrefix(href, "/detail/") {
			matches := detailIDRegex.FindStringSubmatch(href)
			if len(matches) > 1 {
				detailID = matches[1]
			}
		}
		
		if detailID == "" {
			return
		}
		
		detailURL := fmt.Sprintf("%s/detail/%s", SiteBaseURL, detailID)
		result := model.SearchResult{
			UniqueID: fmt.Sprintf("%s-%s-%s", p.Name(), detailID, url.QueryEscape(detailURL)),
			Title:    title,
			Content:  "\u6765\u81ea\u70ed\u641c\u699c\u7684\u63a8\u8350\u8d44\u6e90 | \u6765\u6e90: hot_search",
			Tags:     []string{},
			Links:    []model.Link{},
			Images:   []string{},
			Channel:  "",
			Datetime: time.Time{},
		}
		results = append(results, result)
	})
	
	if len(results) == 0 {
		return nil, fmt.Errorf("\u70ed\u641c\u699c\u672a\u627e\u5230\u5339\u914d\u7ed3\u679c")
	}
	
	fmt.Printf("[%s] \u70ed\u641c\u699c\u627e\u5230 %d \u4e2a\u5339\u914d\u7ed3\u679c\n", p.Name(), len(results))
	return results, nil
}


// convertAPIResultToSearchResult 将 API 返回结构转换为统一 SearchResult。
func (p *PiozAsyncPlugin) convertAPIResultToSearchResult(item struct {
	ID         int    `json:"id"`
	Title      string `json:"title"`
	CloudType  string `json:"cloud_type"`
	Datetime   string `json:"datetime"`
	Size       string `json:"size"`
	Desc       string `json:"desc"`
	CreateTime string `json:"create_time"`
	ViewURL    string `json:"view_url"`
}) model.SearchResult {

	cloudTypeName := p.getCloudTypeName(item.CloudType)
	

	contentParts := []string{}
	if cloudTypeName != "" {
		contentParts = append(contentParts, "\u6765\u6e90: "+cloudTypeName)
	}
	if item.Datetime != "" {
		contentParts = append(contentParts, "\u65f6\u95f4: "+item.Datetime)
	}
	if item.Size != "" {
		contentParts = append(contentParts, "\u5927\u5c0f: "+item.Size)
	}
	if item.Desc != "" {
		contentParts = append(contentParts, "\u63cf\u8ff0: "+item.Desc)
	}
	

	viewURL := item.ViewURL
	if viewURL == "" {
		viewURL = fmt.Sprintf("%s/detail/%d", SiteBaseURL, item.ID)
	}
	
	return model.SearchResult{
		UniqueID: fmt.Sprintf("%s-%d-%s", p.Name(), item.ID, url.QueryEscape(viewURL)),
		Title:    item.Title,
		Content:  strings.Join(contentParts, "\n"),
		Tags:     []string{},
		Links:    []model.Link{},
		Images:   []string{},
		Channel:  "",
		Datetime: time.Time{},
	}
}


// parseSearchItem 解析搜索页中的单个条目，提取标题、详情页 URL、摘要信息。
func (p *PiozAsyncPlugin) parseSearchItem(s *goquery.Selection, keyword string, index int) model.SearchResult {
	result := model.SearchResult{}


	title := ""
	titleSelectors := []string{
		".text-gray-100",
		"[class*='title']",
		".hot-search-title-text",
		"span[title]",
	}
	
	for _, selector := range titleSelectors {
		if titleElem := s.Find(selector).First(); titleElem.Length() > 0 {
			title = strings.TrimSpace(titleElem.Text())
			if title == "" {
				if titleAttr, exists := titleElem.Attr("title"); exists {
					title = strings.TrimSpace(titleAttr)
				}
			}
			if title != "" {
				break
			}
		}
	}

	if title == "" {
		text := strings.TrimSpace(s.Text())
		if len(text) > 100 {
			title = text[:100] + "..."
		} else if text != "" {
			title = text
		}
	}

	if title == "" {
		return result
	}


	var detailURL string
	s.Find("a[href]").Each(func(j int, a *goquery.Selection) {
		if href, exists := a.Attr("href"); exists {
			if strings.Contains(href, "/detail/") {
				if strings.HasPrefix(href, "http") {
					detailURL = href
				} else {
					detailURL = SiteBaseURL + href
				}
				return
			}
		}
	})

	if detailURL == "" && s.Is("a") {
		if href, exists := s.Attr("href"); exists && strings.Contains(href, "/detail/") {
			if strings.HasPrefix(href, "http") {
				detailURL = href
			} else {
				detailURL = SiteBaseURL + href
			}
		}
	}


	content := ""
	s.Find(".text-gray-400, .text-sm, [class*='desc'], [class*='info']").Each(func(j int, elem *goquery.Selection) {
		text := strings.TrimSpace(elem.Text())
		if text != "" && !strings.Contains(text, "\u5938\u514b\u7f51\u76d8") && !strings.Contains(text, "2026-") {
			if content == "" {
				content = text
			} else {
				content += " | " + text
			}
		}
	})

	if content == "" {
		s.Find(".text-gray-400").Each(func(j int, elem *goquery.Selection) {
			text := strings.TrimSpace(elem.Text())
			if strings.Contains(text, "\u5938\u514b\u7f51\u76d8") || strings.Contains(text, "2026-") {
				if content == "" {
					content = text
				} else {
					content += " | " + text
				}
			}
		})
	}


	if detailURL != "" {
		result.UniqueID = fmt.Sprintf("%s-detail-%s", p.Name(), url.QueryEscape(detailURL))
	} else {
		result.UniqueID = fmt.Sprintf("%s-html-%d-%d", p.Name(), index, time.Now().UnixNano())
	}

	result.Title = title
	if content != "" {
		result.Content = content + " | \u6765\u6e90: html_search"
	} else {
		result.Content = "\u6765\u6e90: html_search"
	}
	result.Datetime = time.Time{}
	result.Links = []model.Link{}
	result.Channel = ""

	return result
}


// parseDetailLink 从详情页链接节点直接构建 SearchResult（简化回退分支）。
func (p *PiozAsyncPlugin) parseDetailLink(a *goquery.Selection, index int) model.SearchResult {
	result := model.SearchResult{}
	
	href, exists := a.Attr("href")
	if !exists {
		return result
	}
	

	matches := detailIDRegex.FindStringSubmatch(href)
	if len(matches) < 2 {
		return result
	}
	
	detailID := matches[1]
	title := strings.TrimSpace(a.Text())
	if title == "" {
		title = "\u8d44\u6e90\u8be6\u60c5"
	}
	
	var detailURL string
	if strings.HasPrefix(href, "http") {
		detailURL = href
	} else {
		detailURL = SiteBaseURL + href
	}
	
	result.UniqueID = fmt.Sprintf("%s-%s-%s", p.Name(), detailID, url.QueryEscape(detailURL))
	result.Title = title
	result.Content = "\u6765\u81ea\u641c\u7d22\u7ed3\u679c\u9875 | \u6765\u6e90: html_search"
	result.Links = []model.Link{}
	result.Channel = ""
	result.Datetime = time.Time{}
	
	return result
}


// =========================
// 详情增强（最终链接提取）
// =========================

// enhanceWithDetails 并发增强搜索结果，从详情页/transfer 接口提取真实网盘链接。
func (p *PiozAsyncPlugin) enhanceWithDetails(client *http.Client, results []model.SearchResult) []model.SearchResult {
	if len(results) == 0 {
		return results
	}

	if len(results) > MaxEnhancedResults {
		results = results[:MaxEnhancedResults]
	}
	
	var enhancedResults []model.SearchResult
	var mu sync.Mutex
	var wg sync.WaitGroup
	
	semaphore := make(chan struct{}, MaxConcurrency)
	
	for _, result := range results {
		wg.Add(1)
		
		go func(r model.SearchResult) {
			defer wg.Done()
			
			semaphore <- struct{}{}
			defer func() { <-semaphore }()
			

			p.applyAntiCrawlerDelay()
			

			if cached, ok := detailCache.Load(r.UniqueID); ok {
				if cachedResult, ok := cached.(model.SearchResult); ok {
					mu.Lock()
					enhancedResults = append(enhancedResults, cachedResult)
					mu.Unlock()
					return
				}
			}
			

			links := p.fetchResourceInfo(client, r)
			r.Links = links
			

			if len(links) > 0 {
				fmt.Printf("[%s] \u6210\u529f\u83b7\u53d6\u8d44\u6e90\u94fe\u63a5: %s -> %d\u4e2a\u94fe\u63a5\n", 
					p.Name(), r.Title, len(links))
			}
			

			detailCache.Store(r.UniqueID, r)
			
			mu.Lock()
			enhancedResults = append(enhancedResults, r)
			mu.Unlock()
		}(result)
	}
	
	wg.Wait()
	return enhancedResults
}


// fetchResourceInfo 单条结果增强入口：先 transfer API，再详情页解析。
func (p *PiozAsyncPlugin) fetchResourceInfo(client *http.Client, result model.SearchResult) []model.Link {

	start := time.Now()
	atomic.AddInt64(&detailRequests, 1)
	defer func() {
		duration := time.Since(start).Nanoseconds()
		atomic.AddInt64(&totalDetailTime, duration)
	}()


	links := p.tryTransferAPI(client, result)
	if len(links) > 0 {
		return links
	}
	

	return p.parseResourceDetailPage(client, result)
}

// extractResourceID 从 UniqueID 中提取资源 ID（供 transfer API 使用）。
func (p *PiozAsyncPlugin) extractResourceID(uniqueID string) string {
	if uniqueID == "" {
		return ""
	}

	if strings.Contains(uniqueID, "-detail-") {
		return ""
	}

	parts := strings.SplitN(uniqueID, "-", 3)
	if len(parts) >= 2 {
		return strings.TrimSpace(parts[1])
	}
	return ""
}

// parseDetailURLFromUniqueID 从 UniqueID 反解详情页 URL。
func (p *PiozAsyncPlugin) parseDetailURLFromUniqueID(uniqueID string) string {
	if uniqueID == "" {
		return ""
	}

	if strings.Contains(uniqueID, "-detail-") {
		parts := strings.SplitN(uniqueID, "-detail-", 2)
		if len(parts) == 2 {
			if detailURL, err := url.QueryUnescape(parts[1]); err == nil && detailURL != "" {
				return detailURL
			}
			return parts[1]
		}
	}

	parts := strings.SplitN(uniqueID, "-", 3)
	if len(parts) >= 3 {
		if detailURL, err := url.QueryUnescape(parts[2]); err == nil && detailURL != "" {
			return detailURL
		}
		return parts[2]
	}
	if len(parts) >= 2 {
		return fmt.Sprintf("%s/detail/%s", SiteBaseURL, parts[1])
	}
	return ""
}


// tryTransferAPI 调用 transfer API 尝试直接获取可用分享链接。
func (p *PiozAsyncPlugin) tryTransferAPI(client *http.Client, result model.SearchResult) []model.Link {

	resourceID := p.extractResourceID(result.UniqueID)
	
	if resourceID == "" {
		return nil
	}
	

	cacheKey := fmt.Sprintf("transfer:%s", resourceID)
	if cached, ok := transferCache.Load(cacheKey); ok {
		if links, ok := cached.([]model.Link); ok && len(links) > 0 {
			return links
		}
	}
	

	transferURL := fmt.Sprintf("%s/transfer?id=%s", APIBaseURL, url.QueryEscape(resourceID))
	
	req, err := http.NewRequest("GET", transferURL, nil)
	if err != nil {
		return nil
	}
	
	p.setAPIHeaders(req)
	p.addSessionCookies(req)
	
	resp, err := p.doRequestWithRetry(client, req)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()
	

	if p.checkAntiCrawlerResponse(resp) {
		atomic.AddInt64(&antiCrawlerBlocks, 1)
		return nil
	}
	
	if resp.StatusCode != 200 {
		return nil
	}
	
	body, err := p.readCompressedBody(resp)
	if err != nil {
		return nil
	}
	
	var transferResp TransferResponse
	if err := json.Unmarshal(body, &transferResp); err != nil {
		return nil
	}
	
	if !transferResp.Success || transferResp.Data.URL == "" {
		return nil
	}
	

	link := p.createLinkFromURL(transferResp.Data.URL, transferResp.Data.Password)
	links := []model.Link{link}
	

	transferCache.Store(cacheKey, links)
	
	return links
}


// parseResourceDetailPage 请求详情页并提取页面中的分享链接（transfer 失败时回退）。
func (p *PiozAsyncPlugin) parseResourceDetailPage(client *http.Client, result model.SearchResult) []model.Link {

	detailURL := p.parseDetailURLFromUniqueID(result.UniqueID)
	
	if detailURL == "" {
		return nil
	}
	
	ctx, cancel := context.WithTimeout(context.Background(), DetailTimeout)
	defer cancel()
	
	req, err := http.NewRequestWithContext(ctx, "GET", detailURL, nil)
	if err != nil {
		return nil
	}
	
	p.setStealthHeaders(req)
	p.addSessionCookies(req)
	
	resp, err := client.Do(req)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()
	

	if p.checkAntiCrawlerResponse(resp) {
		atomic.AddInt64(&antiCrawlerBlocks, 1)
		return nil
	}
	
	if resp.StatusCode != 200 {
		return nil
	}
	
	body, err := p.readCompressedBody(resp)
	if err != nil {
		return nil
	}

	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(body))
	if err != nil {
		return nil
	}
	

	return p.extractLinksFromDocument(doc)
}


// extractLinksFromDocument 从详情页文档中提取并去重链接，同时尝试提取提取码。
func (p *PiozAsyncPlugin) extractLinksFromDocument(doc *goquery.Document) []model.Link {
	var links []model.Link
	pageText := doc.Text()
	

	urls := p.extractAllURLs(pageText)
	

	doc.Find("a[href]").Each(func(i int, s *goquery.Selection) {
		href, exists := s.Attr("href")
		if exists && p.isValidNetworkDriveURL(href) {
			urls = append(urls, href)
		}
	})
	
	for _, urlStr := range urls {
		linkType := p.determineLinkType(urlStr)
		if linkType == "" {
			continue
		}
		

		password := p.extractPasswordFromURL(urlStr)
		if password == "" {
			password = p.extractPasswordFromText(pageText)
		}
		
		link := model.Link{
			Type:     linkType,
			URL:      urlStr,
			Password: password,
		}
		

		if !p.containsLink(links, link) {
			links = append(links, link)
		}
	}
	
	return links
}


// extractAllURLs 用正则批量提取文本中的所有网盘链接候选。
func (p *PiozAsyncPlugin) extractAllURLs(text string) []string {
	var urls []string
	

	patterns := []*regexp.Regexp{
		quarkLinkRegex,
		baiduLinkRegex,
		aliyunLinkRegex,
		ucLinkRegex,
		xunleiLinkRegex,
		tianyiLinkRegex,
		lanzouLinkRegex,
		link115Regex,
		mobileLinkRegex,
		weiyunLinkRegex,
		jianguoyunLinkRegex,
		link123Regex,
		pikpakLinkRegex,
		magnetLinkRegex,
		ed2kLinkRegex,
	}
	
	for _, regex := range patterns {
		matches := regex.FindAllString(text, -1)
		urls = append(urls, matches...)
	}
	
	return urls
}

// readCompressedBody 读取压缩响应体，兼容 gzip 与 deflate。
func (p *PiozAsyncPlugin) readCompressedBody(resp *http.Response) ([]byte, error) {
	encoding := strings.ToLower(strings.TrimSpace(resp.Header.Get("Content-Encoding")))
	switch encoding {
	case "gzip":
		gz, err := gzip.NewReader(resp.Body)
		if err != nil {
			return nil, err
		}
		defer gz.Close()
		return io.ReadAll(gz)
	case "deflate":
		raw, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, err
		}

		zr, zErr := zlib.NewReader(bytes.NewReader(raw))
		if zErr == nil {
			defer zr.Close()
			return io.ReadAll(zr)
		}

		fr := flate.NewReader(bytes.NewReader(raw))
		defer fr.Close()
		return io.ReadAll(fr)
	default:
		return io.ReadAll(resp.Body)
	}
}




// =========================
// 反爬与请求层
// =========================

// applyAntiCrawlerDelay 在请求前做动态节流，并定期轮换 UA。
func (p *PiozAsyncPlugin) applyAntiCrawlerDelay() {
	now := time.Now()
	

	timeSinceLast := now.Sub(lastRequestTime)
	

	if timeSinceLast < RequestDelayMin {
		delay := RequestDelayMin - timeSinceLast

		randomDelay := time.Duration(time.Now().UnixNano()%500) * time.Millisecond
		totalDelay := delay + randomDelay
		
		if totalDelay > 0 {
			time.Sleep(totalDelay)
		}
	}
	

	lastRequestTime = time.Now()
	

	requestCount := atomic.AddInt64(&requestCounter, 1)
	if requestCount%5 == 0 {
		randomIndex := time.Now().UnixNano() % int64(len(p.userAgents))
		p.currentUserAgent = p.userAgents[randomIndex]
	}
}


// setStealthHeaders 设置页面请求头（模拟浏览器访问）。
func (p *PiozAsyncPlugin) setStealthHeaders(req *http.Request) {
	req.Header.Set("User-Agent", p.currentUserAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.7")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8,en-GB;q=0.7,en-US;q=0.6")
	req.Header.Set("Accept-Encoding", "gzip, deflate")
	req.Header.Set("Connection", "keep-alive")
	req.Header.Set("Upgrade-Insecure-Requests", "1")
	req.Header.Set("Sec-Fetch-Dest", "document")
	req.Header.Set("Sec-Fetch-Mode", "navigate")
	req.Header.Set("Sec-Fetch-Site", "same-origin")
	req.Header.Set("Sec-Fetch-User", "?1")
	req.Header.Set("Cache-Control", "max-age=0")
	req.Header.Set("Pragma", "no-cache")
	

	req.Header.Set("Sec-Ch-Ua", `"Not_A Brand";v="8", "Chromium";v="120", "Microsoft Edge";v="120"`)
	req.Header.Set("Sec-Ch-Ua-Mobile", "?0")
	req.Header.Set("Sec-Ch-Ua-Platform", `"Windows"`)
	
	if req.URL.Host == "www.pioz.cn" {
		req.Header.Set("Referer", "https://www.pioz.cn/")
	}
}


// setAPIHeaders 设置 API 请求头。
func (p *PiozAsyncPlugin) setAPIHeaders(req *http.Request) {
	req.Header.Set("User-Agent", p.currentUserAgent)
	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")
	req.Header.Set("Accept-Encoding", "gzip, deflate")
	req.Header.Set("Connection", "keep-alive")
	req.Header.Set("X-Requested-With", "XMLHttpRequest")
	req.Header.Set("Sec-Fetch-Dest", "empty")
	req.Header.Set("Sec-Fetch-Mode", "cors")
	req.Header.Set("Sec-Fetch-Site", "same-origin")
	req.Header.Set("Referer", "https://www.pioz.cn/")
	req.Header.Set("Origin", "https://www.pioz.cn")
}


// addSessionCookies 注入会话 Cookie；首次请求补充默认 Cookie。
func (p *PiozAsyncPlugin) addSessionCookies(req *http.Request) {
	sessionMutex.RLock()
	defer sessionMutex.RUnlock()
	
	for _, cookie := range sessionCookies {
		req.AddCookie(cookie)
	}
	

	if len(sessionCookies) == 0 {
		req.AddCookie(&http.Cookie{
			Name:  "first_visit",
			Value: "1",
		})
		req.AddCookie(&http.Cookie{
			Name:  "session_id",
			Value: fmt.Sprintf("%d", time.Now().Unix()),
		})
	}
}


// checkAntiCrawlerResponse 基于状态码和响应头判断是否触发反爬。
func (p *PiozAsyncPlugin) checkAntiCrawlerResponse(resp *http.Response) bool {

	if resp.StatusCode == 403 || resp.StatusCode == 429 {
		return true
	}
	

	if strings.Contains(resp.Header.Get("Server"), "anti-crawler") {
		return true
	}
	
	return false
}


// doRequestWithRetry 带指数退避重试，并同步最新 Cookie。
func (p *PiozAsyncPlugin) doRequestWithRetry(client *http.Client, req *http.Request) (*http.Response, error) {
	var lastErr error
	
	for i := 0; i < RetryCount; i++ {
		if i > 0 {

			backoff := time.Duration(1<<uint(i-1)) * 500 * time.Millisecond
			time.Sleep(backoff)
			

			randomIndex := time.Now().UnixNano() % int64(len(p.userAgents))
			req.Header.Set("User-Agent", p.userAgents[randomIndex])
		}
		
		resp, err := client.Do(req)
		if err == nil {

			sessionMutex.Lock()
			sessionCookies = resp.Cookies()
			sessionMutex.Unlock()
			
			return resp, nil
		}
		
		if resp != nil {
			resp.Body.Close()
		}
		
		lastErr = err
	}
	
	return nil, fmt.Errorf("\u91cd\u8bd5 %d \u6b21\u540e\u5931\u8d25: %w", RetryCount, lastErr)
}


// =========================
// 链接识别与文本提取
// =========================

// isValidNetworkDriveURL 判断 URL 是否为候选网盘链接。
func (p *PiozAsyncPlugin) isValidNetworkDriveURL(url string) bool {
	if strings.Contains(url, "javascript:") || 
	   strings.Contains(url, "#") ||
	   url == "" ||
	   (!strings.HasPrefix(url, "http") && !strings.HasPrefix(url, "magnet:") && !strings.HasPrefix(url, "ed2k:")) {
		return false
	}
	
	return p.determineLinkType(url) != ""
}

// determineLinkType 按正则匹配链接类型。
func (p *PiozAsyncPlugin) determineLinkType(urlStr string) string {
	switch {
	case quarkLinkRegex.MatchString(urlStr):
		return "quark"
	case baiduLinkRegex.MatchString(urlStr):
		return "baidu"
	case aliyunLinkRegex.MatchString(urlStr):
		return "aliyun"
	case ucLinkRegex.MatchString(urlStr):
		return "uc"
	case xunleiLinkRegex.MatchString(urlStr):
		return "xunlei"
	case tianyiLinkRegex.MatchString(urlStr):
		return "tianyi"
	case lanzouLinkRegex.MatchString(urlStr):
		return "lanzou"
	case link115Regex.MatchString(urlStr):
		return "115"
	case mobileLinkRegex.MatchString(urlStr):
		return "mobile"
	case weiyunLinkRegex.MatchString(urlStr):
		return "weiyun"
	case jianguoyunLinkRegex.MatchString(urlStr):
		return "jianguoyun"
	case link123Regex.MatchString(urlStr):
		return "123"
	case pikpakLinkRegex.MatchString(urlStr):
		return "pikpak"
	case magnetLinkRegex.MatchString(urlStr):
		return "magnet"
	case ed2kLinkRegex.MatchString(urlStr):
		return "ed2k"
	default:
		return ""
	}
}

// extractPasswordFromURL 从 URL query 中提取 pwd 参数。
func (p *PiozAsyncPlugin) extractPasswordFromURL(urlStr string) string {
	matches := urlPasswordRegex.FindStringSubmatch(urlStr)
	if len(matches) > 1 {
		return matches[1]
	}
	return ""
}

// extractPasswordFromText 从上下文文本中提取提取码/密码。
func (p *PiozAsyncPlugin) extractPasswordFromText(text string) string {
	matches := passwordRegex.FindStringSubmatch(text)
	if len(matches) > 1 {
		return matches[1]
	}
	return ""
}

// createLinkFromURL 组装 Link 结构，并自动判定链接类型。
func (p *PiozAsyncPlugin) createLinkFromURL(urlStr, password string) model.Link {
	linkType := p.determineLinkType(urlStr)
	if linkType == "" {
		linkType = "other"
	}
	
	return model.Link{
		Type:     linkType,
		URL:      urlStr,
		Password: password,
	}
}

// containsLink 按 URL 去重。
func (p *PiozAsyncPlugin) containsLink(links []model.Link, link model.Link) bool {
	for _, l := range links {
		if l.URL == link.URL {
			return true
		}
	}
	return false
}

// getCloudTypeName 将云盘类型 code 转成人类可读名称。
func (p *PiozAsyncPlugin) getCloudTypeName(cloudType string) string {
	cloudType = strings.ToLower(cloudType)
	
	typeMap := map[string]string{
		"quark":      "\u5938\u514b\u7f51\u76d8",
		"baidu":      "\u767e\u5ea6\u7f51\u76d8",
		"xunlei":     "\u8fc5\u96f7\u4e91\u76d8",
		"aliyun":     "\u963f\u91cc\u4e91\u76d8",
		"uc":         "UC\u7f51\u76d8",
		"lanzou":     "\u84dd\u594f\u4e91",
		"115":        "115\u7f51\u76d8",
		"mobile":     "\u79fb\u52a8\u4e91\u76d8",
		"weiyun":     "\u5fae\u4e91",
		"jianguoyun": "\u575a\u679c\u4e91",
		"123":        "123\u4e91\u76d8",
		"pikpak":     "PikPak",
		"tianyi":     "\u5929\u7ffc\u4e91\u76d8",
		"magnet":     "\u78c1\u529b\u94fe\u63a5",
		"ed2k":       "\u7535\u9a74\u94fe\u63a5",
	}
	
	if name, ok := typeMap[cloudType]; ok {
		return name
	}
	
	return cloudType
}


// =========================
// 运行指标
// =========================

// GetPerformanceStats 返回插件运行时指标，便于观测性能与拦截情况。
func (p *PiozAsyncPlugin) GetPerformanceStats() map[string]interface{} {
	totalSearchRequests := atomic.LoadInt64(&searchRequests)
	totalDetailRequests := atomic.LoadInt64(&detailRequests)
	totalCacheHits := atomic.LoadInt64(&cacheHits)
	totalCacheMisses := atomic.LoadInt64(&cacheMisses)
	totalAntiCrawlerBlocks := atomic.LoadInt64(&antiCrawlerBlocks)
	totalSearchTime := atomic.LoadInt64(&totalSearchTime)
	totalDetailTime := atomic.LoadInt64(&totalDetailTime)

	var avgSearchTime, avgDetailTime, cacheHitRate, blockRate float64
	if totalSearchRequests > 0 {
		avgSearchTime = float64(totalSearchTime) / float64(totalSearchRequests) / 1e6
	}
	if totalDetailRequests > 0 {
		avgDetailTime = float64(totalDetailTime) / float64(totalDetailRequests) / 1e6
	}
	if totalCacheHits+totalCacheMisses > 0 {
		cacheHitRate = float64(totalCacheHits) / float64(totalCacheHits+totalCacheMisses) * 100
	}
	if totalSearchRequests > 0 {
		blockRate = float64(totalAntiCrawlerBlocks) / float64(totalSearchRequests) * 100
	}

	return map[string]interface{}{
		"search_requests":        totalSearchRequests,
		"detail_requests":        totalDetailRequests,
		"cache_hits":             totalCacheHits,
		"cache_misses":           totalCacheMisses,
		"cache_hit_rate":         cacheHitRate,
		"anti_crawler_blocks":    totalAntiCrawlerBlocks,
		"block_rate":             blockRate,
		"avg_search_time_ms":     avgSearchTime,
		"avg_detail_time_ms":     avgDetailTime,
		"total_search_time_ns":   totalSearchTime,
		"total_detail_time_ns":   totalDetailTime,
		"session_cookies":        len(sessionCookies),
		"current_user_agent":     p.currentUserAgent,
	}
}


type cachedResponse struct {
	results   []model.SearchResult
	timestamp time.Time
}
