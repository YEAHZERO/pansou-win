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
	APIBaseURL  = "https://www.pioz.cn/api"
	SiteBaseURL = "https://www.pioz.cn"

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
	plugin.RegisterGlobalPlugin(NewPiozPythonPlugin())
}

var (

	// 网盘链接匹配（支持多种类型）
	quarkLinkRegex         = regexp.MustCompile(`https?://pan\.quark\.cn/s/[0-9a-zA-Z]{6,}`)
	quarkLinkNoSchemeRegex = regexp.MustCompile(`(?i)\bpan\.quark\.cn/s/[0-9a-zA-Z]{6,}\b`)
	baiduLinkRegex         = regexp.MustCompile(`https?://pan\.baidu\.com/s/[0-9a-zA-Z_\-]+(?:\?pwd=[0-9a-zA-Z]+)?`)
	aliyunLinkRegex        = regexp.MustCompile(`https?://(?:www\.)?(?:aliyundrive\.com|alipan\.com)/s/[0-9a-zA-Z]+`)
	ucLinkRegex            = regexp.MustCompile(`https?://drive\.uc\.cn/s/[0-9a-zA-Z]+(?:\?[^"'\s]*)?`)
	xunleiLinkRegex        = regexp.MustCompile(`https?://pan\.xunlei\.com/s/[0-9a-zA-Z_\-]+(?:\?pwd=[0-9a-zA-Z]+)?`)
	tianyiLinkRegex        = regexp.MustCompile(`https?://cloud\.189\.cn/t/[0-9a-zA-Z]+`)
	lanzouLinkRegex        = regexp.MustCompile(`https?://(?:www\.)?(?:lanzou[uixys]*|lan[zs]o[ux])\.(?:com|net|org)/[0-9a-zA-Z]+`)
	link115Regex           = regexp.MustCompile(`https?://115\.com/s/[0-9a-zA-Z]+`)
	mobileLinkRegex        = regexp.MustCompile(`https?://caiyun\.feixin\.10086\.cn/[0-9a-zA-Z]+`)
	weiyunLinkRegex        = regexp.MustCompile(`https?://share\.weiyun\.com/[0-9a-zA-Z]+`)
	jianguoyunLinkRegex    = regexp.MustCompile(`https?://(?:www\.)?jianguoyun\.com/p/[0-9a-zA-Z]+`)
	link123Regex           = regexp.MustCompile(`https?://123pan\.com/s/[0-9a-zA-Z]+`)
	pikpakLinkRegex        = regexp.MustCompile(`https?://mypikpak\.com/s/[0-9a-zA-Z]+`)
	magnetLinkRegex        = regexp.MustCompile(`magnet:\?xt=urn:btih:[0-9a-fA-F]{40}`)
	ed2kLinkRegex          = regexp.MustCompile(`ed2k://\|file\|.+\|\d+\|[0-9a-fA-F]{32}\|/`)
	escapedHTTPURLRegex    = regexp.MustCompile(`https?:\\/?\\/?[^\s"'<>]+`)
	quotedPathRegex        = regexp.MustCompile(`["'](\/[^"']{2,})["']`)
	locationAssignRegex    = regexp.MustCompile(`(?i)(?:location\.href|window\.location|window\.open|url)\s*[:=]\s*["']([^"']+)["']`)
	jsInvokeRegex          = regexp.MustCompile(`([a-zA-Z_][a-zA-Z0-9_]*)\(([^)]*)\)`)

	// 提取码/密码匹配：支持“提取码/密码/pwd/码: XXXX”
	passwordRegex    = regexp.MustCompile("(?i)(?:\u63d0\u53d6\u7801|\u5bc6\u7801|pwd|\u7801)[\uff1a:]\\s*([a-zA-Z0-9]{4})")
	urlPasswordRegex = regexp.MustCompile(`(?i)\?pwd=([0-9a-zA-Z]+)`)

	// 详情页 ID 提取：/detail/{id}
	detailIDRegex = regexp.MustCompile(`/detail/(\d+)`)

	// 反爬关键词：页面/响应体出现这些字样时判定为反爬拦截
	antiCrawlerRegex = regexp.MustCompile("\u7981\u6b62\u4f7f\u7528\u5f00\u53d1\u8005\u5de5\u5177|\u5077\u6837\u5f0f\u6b7b\u5168\u5bb6|\u53cd\u722c\u866b|\u9632\u722c")
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
	searchCache = sync.Map{}
	// 详情缓存：UniqueID -> SearchResult(with Links)
	detailCache = sync.Map{}
	// transfer 缓存：resourceID -> []Link
	transferCache = sync.Map{}

	sessionCookies  []*http.Cookie
	sessionMutex    sync.RWMutex
	lastRequestTime time.Time
	requestCounter  int64
)

var (
	searchRequests    int64 = 0
	detailRequests    int64 = 0
	cacheHits         int64 = 0
	cacheMisses       int64 = 0
	antiCrawlerBlocks int64 = 0
	totalSearchTime   int64 = 0
	totalDetailTime   int64 = 0
)

// =========================
// 插件定义
// =========================

type PiozAsyncPlugin struct {
	*plugin.BaseAsyncPlugin
	optimizedClient  *http.Client
	userAgents       []string
	currentUserAgent string
	linkPool         sync.Pool
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
		linkPool: sync.Pool{
			New: func() interface{} {
				return &model.Link{}
			},
		},
	}
}

func (p *PiozAsyncPlugin) getLink() *model.Link {
	return p.linkPool.Get().(*model.Link)
}

func (p *PiozAsyncPlugin) putLink(link *model.Link) {
	link.Type = ""
	link.URL = ""
	link.Password = ""
	p.linkPool.Put(link)
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

	doc.Find("a[href^='/detail/']").Each(func(i int, s *goquery.Selection) {
		result := p.parseSearchItemNew(s, keyword, i)
		if result.UniqueID != "" {
			results = append(results, result)
		}
	})

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

// parseSearchItemNew 解析新版本搜索页中的单个条目，提取标题、详情页 URL、摘要信息。
func (p *PiozAsyncPlugin) parseSearchItemNew(s *goquery.Selection, keyword string, index int) model.SearchResult {
	result := model.SearchResult{}

	href, exists := s.Attr("href")
	if !exists {
		return result
	}

	if !strings.Contains(href, "/detail/") {
		return result
	}

	detailURL := href
	if !strings.HasPrefix(href, "http") {
		detailURL = SiteBaseURL + href
	}

	title := strings.TrimSpace(s.Find(".truncate, span").First().Text())
	if title == "" {
		title = strings.TrimSpace(s.Text())
	}

	if title == "" {
		return result
	}

	var contentParts []string

	s.Find(".text-muted-foreground, span").Each(func(j int, elem *goquery.Selection) {
		text := strings.TrimSpace(elem.Text())
		if text != "" && !strings.Contains(text, title) {
			contentParts = append(contentParts, text)
		}
	})

	content := strings.Join(contentParts, " | ")
	if content == "" {
		content = "\u6765\u6e90: html_search"
	}

	result.UniqueID = fmt.Sprintf("%s-detail-%s", p.Name(), url.QueryEscape(detailURL))
	result.Title = title
	result.Content = content
	result.Datetime = time.Time{}
	result.Links = []model.Link{}
	result.Channel = ""

	return result
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

	withLinks := 0
	for _, r := range enhancedResults {
		if len(r.Links) > 0 {
			withLinks++
		}
	}
	fmt.Printf("[%s] 详情增强完成: 输入=%d, 输出=%d, 含链接=%d\n", p.Name(), len(results), len(enhancedResults), withLinks)
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
		fmt.Printf("[%s] transfer命中: title=%s links=%d\n", p.Name(), result.Title, len(links))
		return links
	}

	links = p.parseResourceDetailPage(client, result)
	if len(links) > 0 {
		fmt.Printf("[%s] detail命中: title=%s links=%d\n", p.Name(), result.Title, len(links))
	} else {
		fmt.Printf("[%s] detail未命中: title=%s uniqueID=%s\n", p.Name(), result.Title, result.UniqueID)
	}
	return links
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
		// Fallback: parse detail URL and recover ID from /detail/{id}.
		detailURL := p.parseDetailURLFromUniqueID(result.UniqueID)
		if detailURL != "" {
			if matches := detailIDRegex.FindStringSubmatch(detailURL); len(matches) > 1 {
				resourceID = matches[1]
			}
		}
	}

	if resourceID == "" {
		fmt.Printf("[%s] transfer跳过: 无resourceID uniqueID=%s\n", p.Name(), result.UniqueID)
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
		fmt.Printf("[%s] transfer失败: request err=%v id=%s\n", p.Name(), err, resourceID)
		return nil
	}
	defer resp.Body.Close()

	if p.checkAntiCrawlerResponse(resp) {
		atomic.AddInt64(&antiCrawlerBlocks, 1)
		return nil
	}

	if resp.StatusCode != 200 {
		fmt.Printf("[%s] transfer失败: status=%d id=%s\n", p.Name(), resp.StatusCode, resourceID)
		// Fallback: endpoint might have changed; probe common API variants.
		if resp.StatusCode == http.StatusNotFound {
			if links := p.tryTransferEndpointFallbacks(client, resourceID); len(links) > 0 {
				transferCache.Store(cacheKey, links)
				fmt.Printf("[%s] transfer兜底命中: id=%s links=%d\n", p.Name(), resourceID, len(links))
				return links
			}
		}
		return nil
	}

	body, err := p.readCompressedBody(resp)
	if err != nil {
		return nil
	}

	var transferResp TransferResponse
	if err := json.Unmarshal(body, &transferResp); err != nil {
		fmt.Printf("[%s] transfer失败: unmarshal err=%v id=%s body=%s\n", p.Name(), err, resourceID, string(body))
		return nil
	}

	if !transferResp.Success || transferResp.Data.URL == "" {
		fmt.Printf("[%s] transfer无结果: success=%v id=%s body=%s\n", p.Name(), transferResp.Success, resourceID, string(body))
		return nil
	}

	link := p.createLinkFromURL(transferResp.Data.URL, transferResp.Data.Password)
	links := []model.Link{link}

	// Fallback: transfer may return an intermediate redirect URL.
	if !p.isValidNetworkDriveURL(link.URL) {
		if redirected := p.resolveRedirectShareLinksWithReferer(client, link.URL, SiteBaseURL); len(redirected) > 0 {
			for i := range redirected {
				if redirected[i].Password == "" {
					redirected[i].Password = transferResp.Data.Password
				}
			}
			links = redirected
		}
	}

	transferCache.Store(cacheKey, links)
	fmt.Printf("[%s] transfer成功: id=%s url=%s\n", p.Name(), resourceID, links[0].URL)

	return links
}

// tryTransferEndpointFallbacks 探测常见 transfer 类接口变体（路径/方法），用于应对站点接口改版。
func (p *PiozAsyncPlugin) tryTransferEndpointFallbacks(client *http.Client, resourceID string) []model.Link {
	type endpointProbe struct {
		method      string
		url         string
		contentType string
		body        string
	}

	apiCandidates := []string{
		fmt.Sprintf("%s/transfer?id=%s", APIBaseURL, url.QueryEscape(resourceID)),
		fmt.Sprintf("%s/transfer/%s", APIBaseURL, url.PathEscape(resourceID)),
		fmt.Sprintf("%s/resource/transfer?id=%s", APIBaseURL, url.QueryEscape(resourceID)),
		fmt.Sprintf("%s/detail/transfer?id=%s", APIBaseURL, url.QueryEscape(resourceID)),
		fmt.Sprintf("%s/share?id=%s", APIBaseURL, url.QueryEscape(resourceID)),
		fmt.Sprintf("%s/getShare?id=%s", APIBaseURL, url.QueryEscape(resourceID)),
		fmt.Sprintf("%s/link?id=%s", APIBaseURL, url.QueryEscape(resourceID)),
		fmt.Sprintf("%s/source?id=%s", APIBaseURL, url.QueryEscape(resourceID)),
	}

	probes := make([]endpointProbe, 0, len(apiCandidates)*3)
	for _, u := range apiCandidates {
		probes = append(probes, endpointProbe{method: "GET", url: u})
		probes = append(probes, endpointProbe{
			method:      "POST",
			url:         u,
			contentType: "application/x-www-form-urlencoded",
			body:        "id=" + url.QueryEscape(resourceID),
		})
		probes = append(probes, endpointProbe{
			method:      "POST",
			url:         u,
			contentType: "application/json",
			body:        fmt.Sprintf(`{"id":"%s","resource_id":"%s"}`, resourceID, resourceID),
		})
	}

	for _, probe := range probes {
		var bodyReader io.Reader
		if probe.body != "" {
			bodyReader = strings.NewReader(probe.body)
		}

		req, err := http.NewRequest(probe.method, probe.url, bodyReader)
		if err != nil {
			continue
		}
		p.setAPIHeaders(req)
		if probe.contentType != "" {
			req.Header.Set("Content-Type", probe.contentType)
		}
		p.addSessionCookies(req)

		resp, err := p.doRequestWithRetry(client, req)
		if err != nil {
			continue
		}

		raw, readErr := p.readCompressedBody(resp)
		resp.Body.Close()
		if readErr != nil || len(raw) == 0 {
			continue
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			continue
		}

		links := p.extractLinksFromAnyPayload(raw)
		if len(links) > 0 {
			fmt.Printf("[%s] transfer兜底接口命中: %s %s links=%d\n", p.Name(), probe.method, probe.url, len(links))
			return links
		}
	}

	return nil
}

// extractLinksFromAnyPayload 从 JSON/文本响应中尽可能提取网盘链接。
func (p *PiozAsyncPlugin) extractLinksFromAnyPayload(payload []byte) []model.Link {
	text := string(payload)
	urls := p.extractAllURLs(text)

	var parsed interface{}
	if err := json.Unmarshal(payload, &parsed); err == nil {
		flat := p.flattenStringValues(parsed)
		for _, s := range flat {
			urls = append(urls, p.extractAllURLs(s)...)
		}
	}

	seen := make(map[string]struct{}, len(urls))
	links := make([]model.Link, 0, len(urls))
	for _, u := range urls {
		if !p.isValidNetworkDriveURL(u) {
			continue
		}
		if _, ok := seen[u]; ok {
			continue
		}
		seen[u] = struct{}{}
		pw := p.extractPasswordFromURL(u)
		if pw == "" {
			pw = p.extractPasswordFromText(text)
		}
		links = append(links, p.createLinkFromURL(u, pw))
	}
	return links
}

// flattenStringValues 递归提取任意 JSON 结构里的字符串值。
func (p *PiozAsyncPlugin) flattenStringValues(v interface{}) []string {
	var out []string
	var walk func(interface{})
	walk = func(node interface{}) {
		switch t := node.(type) {
		case string:
			if t != "" {
				out = append(out, t)
			}
		case map[string]interface{}:
			for _, vv := range t {
				walk(vv)
			}
		case []interface{}:
			for _, vv := range t {
				walk(vv)
			}
		}
	}
	walk(v)
	return out
}

// parseResourceDetailPage 请求详情页并提取页面中的分享链接（transfer 失败时回退）。
func (p *PiozAsyncPlugin) parseResourceDetailPage(client *http.Client, result model.SearchResult) []model.Link {

	detailURL := p.parseDetailURLFromUniqueID(result.UniqueID)

	if detailURL == "" {
		fmt.Printf("[%s] detail跳过: 无detailURL uniqueID=%s\n", p.Name(), result.UniqueID)
		return nil
	}

	// Try redirect chain first (some pages jump to share links via 302).
	if redirected := p.resolveRedirectShareLinksWithReferer(client, detailURL, SiteBaseURL); len(redirected) > 0 {
		return redirected
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
		fmt.Printf("[%s] detail失败: request err=%v url=%s\n", p.Name(), err, detailURL)
		return nil
	}
	defer resp.Body.Close()

	if p.checkAntiCrawlerResponse(resp) {
		atomic.AddInt64(&antiCrawlerBlocks, 1)
		return nil
	}

	if resp.StatusCode != 200 {
		fmt.Printf("[%s] detail失败: status=%d url=%s\n", p.Name(), resp.StatusCode, detailURL)
		return nil
	}

	body, err := p.readCompressedBody(resp)
	if err != nil {
		return nil
	}

	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(body))
	if err != nil {
		fmt.Printf("[%s] detail失败: parse err=%v url=%s\n", p.Name(), err, detailURL)
		return nil
	}

	links := p.extractLinksFromDocument(doc)
	if len(links) == 0 {
		// Fallback: emulate consent click/submit flow when page requires "了解并同意获取".
		if consentLinks := p.tryConsentFlowNew(client, doc, detailURL); len(consentLinks) > 0 {
			for _, link := range consentLinks {
				if !p.containsLink(links, link) {
					links = append(links, link)
				}
			}
		}
	}
	if len(links) == 0 {
		// Fallback: try to resolve intermediate jump URLs from detail page attributes/scripts.
		candidates := p.extractRedirectCandidatesFromDocument(doc, detailURL)
		for _, candidate := range candidates {
			if redirected := p.resolveRedirectShareLinksWithReferer(client, candidate, detailURL); len(redirected) > 0 {
				for _, link := range redirected {
					if !p.containsLink(links, link) {
						links = append(links, link)
					}
				}
			}
		}
	}
	fmt.Printf("[%s] detail解析: url=%s links=%d\n", p.Name(), detailURL, len(links))
	return links
}

// tryConsentFlowNew 尝试自动通过"了解并同意获取"中转页（新版本），再提取最终网盘链接。
func (p *PiozAsyncPlugin) tryConsentFlowNew(client *http.Client, doc *goquery.Document, detailURL string) []model.Link {
	pageText := strings.TrimSpace(doc.Text())
	if pageText == "" {
		return nil
	}

	// 只在明显的同意页触发，避免无谓请求。
	if !strings.Contains(pageText, "了解并同意获取") &&
		!strings.Contains(pageText, "点击获取") &&
		!strings.Contains(pageText, "免责声明") {
		return nil
	}
	fmt.Printf("[%s] 识别到同意页(新版本): %s\n", p.Name(), detailURL)

	var links []model.Link

	// 1) 先尝试直接从页面中提取网盘链接
	links = p.extractLinksFromDocument(doc)
	if len(links) > 0 {
		// 过滤只保留 https://pan.quark.cn/s/ 格式的链接
		var filteredLinks []model.Link
		for _, link := range links {
			if strings.HasPrefix(link.URL, "https://pan.quark.cn/s/") {
				filteredLinks = append(filteredLinks, link)
			}
		}
		if len(filteredLinks) > 0 {
			fmt.Printf("[%s] 同意页直接提取到夸克链接: %d个\n", p.Name(), len(filteredLinks))
			return filteredLinks
		}
	}

	// 2) 尝试 form 提交
	formLinks := p.submitConsentForms(client, doc, detailURL)
	if len(formLinks) > 0 {
		fmt.Printf("[%s] 同意页表单提交成功: links=%d url=%s\n", p.Name(), len(formLinks), detailURL)
	}
	for _, link := range formLinks {
		if strings.HasPrefix(link.URL, "https://pan.quark.cn/s/") {
			if !p.containsLink(links, link) {
				links = append(links, link)
			}
		}
	}
	if len(links) > 0 {
		return links
	}

	// 3) 尝试按钮点击模拟
	buttonLinks := p.clickConsentButtons(client, doc, detailURL)
	if len(buttonLinks) > 0 {
		fmt.Printf("[%s] 同意页按钮点击成功: links=%d url=%s\n", p.Name(), len(buttonLinks), detailURL)
	}
	for _, link := range buttonLinks {
		if strings.HasPrefix(link.URL, "https://pan.quark.cn/s/") {
			if !p.containsLink(links, link) {
				links = append(links, link)
			}
		}
	}

	return links
}

// clickConsentButtons 模拟点击同意按钮，返回可提取到的网盘链接。
func (p *PiozAsyncPlugin) clickConsentButtons(client *http.Client, doc *goquery.Document, detailURL string) []model.Link {
	var links []model.Link

	doc.Find("button").Each(func(i int, button *goquery.Selection) {
		buttonText := strings.TrimSpace(button.Text())
		if !strings.Contains(buttonText, "了解并同意获取") &&
			!strings.Contains(buttonText, "获取资源") &&
			!strings.Contains(buttonText, "点击获取") {
			return
		}

		// 尝试查找按钮的 onclick 事件或 data 属性中的 URL
		if onclick, exists := button.Attr("onclick"); exists {
			urls := p.extractAllURLs(onclick)
			for _, u := range urls {
				if strings.HasPrefix(u, "https://pan.quark.cn/s/") {
					link := p.createLinkFromURL(u, "")
					if !p.containsLink(links, link) {
						links = append(links, link)
					}
				}
			}
		}

		// 检查 data-url, data-href 等属性
		for _, attr := range []string{"data-url", "data-href", "data-link"} {
			if url, exists := button.Attr(attr); exists {
				if strings.HasPrefix(url, "https://pan.quark.cn/s/") {
					link := p.createLinkFromURL(url, "")
					if !p.containsLink(links, link) {
						links = append(links, link)
					}
				}
			}
		}
	})

	return links
}

// tryConsentFlow 尝试自动通过"了解并同意获取"中转页，再提取最终网盘链接。
func (p *PiozAsyncPlugin) tryConsentFlow(client *http.Client, doc *goquery.Document, detailURL string) []model.Link {
	pageText := strings.TrimSpace(doc.Text())
	if pageText == "" {
		return nil
	}

	// 只在明显的同意页触发，避免无谓请求。
	if !strings.Contains(pageText, "了解并同意获取") &&
		!strings.Contains(pageText, "点击获取") &&
		!strings.Contains(pageText, "免责声明") {
		return nil
	}
	fmt.Printf("[%s] 识别到同意页: %s\n", p.Name(), detailURL)
	fmt.Printf("[%s] 同意页结构: form=%d button=%d a=%d\n", p.Name(), doc.Find("form").Length(), doc.Find("button").Length(), doc.Find("a").Length())

	var links []model.Link

	// 1) 先尝试 form 提交。
	formLinks := p.submitConsentForms(client, doc, detailURL)
	if len(formLinks) > 0 {
		fmt.Printf("[%s] 同意页表单提交成功: links=%d url=%s\n", p.Name(), len(formLinks), detailURL)
	}
	for _, link := range formLinks {
		if !p.containsLink(links, link) {
			links = append(links, link)
		}
	}
	if len(links) > 0 {
		return links
	}

	// 2) 再尝试按钮/属性中的跳转 URL。
	candidates := p.extractRedirectCandidatesFromDocument(doc, detailURL)
	if len(candidates) == 0 {
		// If page is JS-rendered, try common jump URL templates based on detail ID.
		if matches := detailIDRegex.FindStringSubmatch(detailURL); len(matches) > 1 {
			id := matches[1]
			candidates = append(candidates,
				fmt.Sprintf("%s/go/%s", SiteBaseURL, id),
				fmt.Sprintf("%s/jump/%s", SiteBaseURL, id),
				fmt.Sprintf("%s/redirect/%s", SiteBaseURL, id),
				fmt.Sprintf("%s/link/%s", SiteBaseURL, id),
				fmt.Sprintf("%s/get/%s", SiteBaseURL, id),
				fmt.Sprintf("%s/api/go/%s", SiteBaseURL, id),
				fmt.Sprintf("%s/api/jump/%s", SiteBaseURL, id),
				fmt.Sprintf("%s/api/redirect/%s", SiteBaseURL, id),
				fmt.Sprintf("%s/api/go?id=%s", SiteBaseURL, url.QueryEscape(id)),
				fmt.Sprintf("%s/api/jump?id=%s", SiteBaseURL, url.QueryEscape(id)),
			)
		}
	}
	fmt.Printf("[%s] 同意页候选跳转: %d url=%s\n", p.Name(), len(candidates), detailURL)
	if len(candidates) > 0 {
		limit := len(candidates)
		if limit > 5 {
			limit = 5
		}
		for i := 0; i < limit; i++ {
			fmt.Printf("[%s] 同意页候选[%d]=%s\n", p.Name(), i, candidates[i])
		}
	}
	for _, candidate := range candidates {
		if redirected := p.resolveRedirectShareLinksWithReferer(client, candidate, detailURL); len(redirected) > 0 {
			for _, link := range redirected {
				if !p.containsLink(links, link) {
					links = append(links, link)
				}
			}
		}
		if len(links) == 0 {
			if probed := p.probeJumpEndpointVariants(client, candidate, detailURL); len(probed) > 0 {
				for _, link := range probed {
					if !p.containsLink(links, link) {
						links = append(links, link)
					}
				}
			}
		}
	}

	return links
}

// submitConsentForms 提交详情页中的同意表单，返回可提取到的网盘链接。
func (p *PiozAsyncPlugin) submitConsentForms(client *http.Client, doc *goquery.Document, detailURL string) []model.Link {
	var links []model.Link
	baseURL, _ := url.Parse(detailURL)

	doc.Find("form").Each(func(i int, form *goquery.Selection) {
		action := strings.TrimSpace(form.AttrOr("action", ""))
		method := strings.ToUpper(strings.TrimSpace(form.AttrOr("method", "GET")))
		if method == "" {
			method = "GET"
		}

		target := detailURL
		if action != "" && baseURL != nil {
			if u, err := url.Parse(action); err == nil && u != nil {
				target = baseURL.ResolveReference(u).String()
			}
		}
		if target == "" {
			return
		}

		values := url.Values{}
		form.Find("input,textarea,select,button").Each(func(_ int, field *goquery.Selection) {
			name := strings.TrimSpace(field.AttrOr("name", ""))
			if name == "" {
				return
			}
			value := field.AttrOr("value", "")
			fieldType := strings.ToLower(strings.TrimSpace(field.AttrOr("type", "")))
			if (fieldType == "checkbox" || fieldType == "radio") && !field.Is("[checked]") {
				return
			}
			if value == "" {
				switch fieldType {
				case "checkbox", "radio":
					value = "on"
				case "submit":
					value = "1"
				}
			}
			values.Set(name, value)
		})

		if len(values) == 0 {
			values.Set("agree", "1")
			values.Set("consent", "1")
			values.Set("confirm", "1")
			values.Set("submit", "1")
		}

		fmt.Printf("[%s] 同意页提交: method=%s action=%s fields=%d\n", p.Name(), method, target, len(values))

		var req *http.Request
		var err error
		if method == "POST" {
			req, err = http.NewRequest("POST", target, strings.NewReader(values.Encode()))
			if err != nil {
				return
			}
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		} else {
			u, e := url.Parse(target)
			if e != nil || u == nil {
				return
			}
			q := u.Query()
			for k, v := range values {
				for _, vv := range v {
					q.Set(k, vv)
				}
			}
			u.RawQuery = q.Encode()
			req, err = http.NewRequest("GET", u.String(), nil)
			if err != nil {
				return
			}
		}

		p.setStealthHeaders(req)
		p.addSessionCookies(req)

		resp, err := p.doRequestWithRetry(client, req)
		if err != nil {
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode >= 300 && resp.StatusCode < 400 {
			location := strings.TrimSpace(resp.Header.Get("Location"))
			fmt.Printf("[%s] 同意页返回302: location=%s\n", p.Name(), location)
			if location != "" {
				next, err := req.URL.Parse(location)
				if err == nil && next != nil {
					if redirected := p.resolveRedirectShareLinks(client, next.String()); len(redirected) > 0 {
						for _, link := range redirected {
							if !p.containsLink(links, link) {
								links = append(links, link)
							}
						}
					}
				}
			}
			return
		}

		if resp.StatusCode != http.StatusOK {
			fmt.Printf("[%s] 同意页返回: status=%d\n", p.Name(), resp.StatusCode)
			return
		}

		body, err := p.readCompressedBody(resp)
		if err != nil {
			return
		}
		text := string(body)
		for _, u := range p.extractAllURLs(text) {
			if p.isValidNetworkDriveURL(u) {
				link := p.createLinkFromURL(u, p.extractPasswordFromText(text))
				if !p.containsLink(links, link) {
					links = append(links, link)
				}
			}
		}
	})

	return links
}

// probeJumpEndpointVariants 对可疑跳转接口做 GET/POST 变体探测，适配前端按钮触发型页面。
func (p *PiozAsyncPlugin) probeJumpEndpointVariants(client *http.Client, candidateURL, detailURL string) []model.Link {
	if candidateURL == "" {
		return nil
	}

	detailID := ""
	if matches := detailIDRegex.FindStringSubmatch(detailURL); len(matches) > 1 {
		detailID = matches[1]
	}

	paramSets := []url.Values{
		{"id": []string{detailID}, "agree": []string{"1"}, "confirm": []string{"1"}},
		{"rid": []string{detailID}, "agree": []string{"1"}, "confirm": []string{"1"}},
		{"post_id": []string{detailID}, "agree": []string{"1"}, "confirm": []string{"1"}},
		{"resource_id": []string{detailID}, "agree": []string{"1"}, "confirm": []string{"1"}},
	}

	for _, vals := range paramSets {
		if detailID == "" {
			delete(vals, "id")
			delete(vals, "rid")
			delete(vals, "post_id")
			delete(vals, "resource_id")
		}

		// GET probe
		if u, err := url.Parse(candidateURL); err == nil && u != nil {
			q := u.Query()
			for k, v := range vals {
				for _, vv := range v {
					if vv != "" {
						q.Set(k, vv)
					}
				}
			}
			u.RawQuery = q.Encode()
			req, err := http.NewRequest("GET", u.String(), nil)
			if err == nil {
				p.setStealthHeaders(req)
				req.Header.Set("Referer", detailURL)
				req.Header.Set("X-Requested-With", "XMLHttpRequest")
				p.addSessionCookies(req)

				resp, err := p.doRequestWithRetry(client, req)
				if err == nil {
					body, readErr := p.readCompressedBody(resp)
					resp.Body.Close()
					if readErr == nil {
						if links := p.extractLinksFromAnyPayload(body); len(links) > 0 {
							fmt.Printf("[%s] 候选接口GET命中: %s links=%d\n", p.Name(), u.String(), len(links))
							return links
						}
					}
				}
			}
		}

		// POST form probe
		req, err := http.NewRequest("POST", candidateURL, strings.NewReader(vals.Encode()))
		if err != nil {
			continue
		}
		p.setStealthHeaders(req)
		req.Header.Set("Referer", detailURL)
		req.Header.Set("X-Requested-With", "XMLHttpRequest")
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded; charset=UTF-8")
		p.addSessionCookies(req)

		resp, err := p.doRequestWithRetry(client, req)
		if err != nil {
			continue
		}
		body, readErr := p.readCompressedBody(resp)
		resp.Body.Close()
		if readErr != nil {
			continue
		}
		if links := p.extractLinksFromAnyPayload(body); len(links) > 0 {
			fmt.Printf("[%s] 候选接口POST命中: %s links=%d\n", p.Name(), candidateURL, len(links))
			return links
		}
	}

	return nil
}

func (p *PiozAsyncPlugin) resolveRedirectShareLinks(client *http.Client, startURL string) []model.Link {
	return p.resolveRedirectShareLinksWithReferer(client, startURL, "")
}

func (p *PiozAsyncPlugin) resolveRedirectShareLinksWithReferer(client *http.Client, startURL, referer string) []model.Link {
	if startURL == "" {
		return nil
	}

	current := startURL
	visited := make(map[string]struct{}, 8)

	for i := 0; i < 6; i++ {
		if _, ok := visited[current]; ok {
			return nil
		}
		visited[current] = struct{}{}

		ctx, cancel := context.WithTimeout(context.Background(), DetailTimeout)
		req, err := http.NewRequestWithContext(ctx, "GET", current, nil)
		if err != nil {
			cancel()
			return nil
		}
		p.setStealthHeaders(req)
		if referer != "" {
			req.Header.Set("Referer", referer)
		}
		p.addSessionCookies(req)

		resp, err := client.Do(req)
		if err != nil {
			cancel()
			return nil
		}

		if resp.StatusCode >= 300 && resp.StatusCode < 400 {
			location := strings.TrimSpace(resp.Header.Get("Location"))
			resp.Body.Close()
			cancel()
			if location == "" {
				return nil
			}

			nextURL, err := req.URL.Parse(location)
			if err != nil || nextURL == nil {
				return nil
			}
			referer = current
			current = nextURL.String()
			if p.isValidNetworkDriveURL(current) {
				return []model.Link{p.createLinkFromURL(current, "")}
			}
			continue
		}

		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			cancel()
			return nil
		}

		body, err := p.readCompressedBody(resp)
		resp.Body.Close()
		cancel()
		if err != nil {
			return nil
		}

		pageText := string(body)
		urls := p.extractAllURLs(pageText)
		for _, u := range urls {
			// 只保留 https://pan.quark.cn/s/ 格式的链接
			if strings.HasPrefix(u, "https://pan.quark.cn/s/") {
				return []model.Link{p.createLinkFromURL(u, p.extractPasswordFromText(pageText))}
			}
		}

		// Not a direct share page yet: attempt one more consent/jump extraction recursively.
		doc, err := goquery.NewDocumentFromReader(bytes.NewReader(body))
		if err == nil {
			if consentLinks := p.tryConsentFlow(client, doc, current); len(consentLinks) > 0 {
				return consentLinks
			}
			candidates := p.extractRedirectCandidatesFromDocument(doc, current)
			for _, candidate := range candidates {
				if _, ok := visited[candidate]; ok {
					continue
				}
				if redirected := p.resolveRedirectShareLinksWithReferer(client, candidate, current); len(redirected) > 0 {
					return redirected
				}
			}
		}
		return nil
	}

	return nil
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

	// Extract URLs embedded in script blocks.
	doc.Find("script").Each(func(i int, s *goquery.Selection) {
		scriptText := s.Text()
		if scriptText != "" {
			urls = append(urls, p.extractAllURLs(scriptText)...)
		}
	})

	// Extract URLs from common data attributes and onclick handlers.
	attrNames := []string{"href", "data-href", "data-url", "data-link", "value"}
	doc.Find("*").Each(func(i int, s *goquery.Selection) {
		for _, attr := range attrNames {
			if v, ok := s.Attr(attr); ok {
				urls = append(urls, p.extractAllURLs(v)...)
				if p.isValidNetworkDriveURL(v) {
					urls = append(urls, v)
				}
			}
		}
		if onclick, ok := s.Attr("onclick"); ok && onclick != "" {
			urls = append(urls, p.extractAllURLs(onclick)...)
		}
	})

	for _, urlStr := range urls {
		// 只保留 https://pan.quark.cn/s/ 格式的链接
		if !strings.HasPrefix(urlStr, "https://pan.quark.cn/s/") {
			continue
		}

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

// extractRedirectCandidatesFromDocument 收集详情页中可能的中转地址（如 /go/xxx、/jump?url=...）。
func (p *PiozAsyncPlugin) extractRedirectCandidatesFromDocument(doc *goquery.Document, detailURL string) []string {
	candidates := make([]string, 0, 16)
	seen := make(map[string]struct{}, 16)

	base, _ := url.Parse(detailURL)
	addCandidate := func(raw string) {
		raw = strings.TrimSpace(raw)
		if raw == "" || strings.HasPrefix(strings.ToLower(raw), "javascript:") || raw == "#" {
			return
		}

		raw = p.normalizeEscapedTextForURLExtraction(raw)

		// If a direct share URL appears after normalization, keep it.
		for _, u := range p.extractAllURLs(raw) {
			if p.isValidNetworkDriveURL(u) {
				if _, ok := seen[u]; !ok {
					seen[u] = struct{}{}
					candidates = append(candidates, u)
				}
			}
		}

		u, err := url.Parse(raw)
		if err != nil || u == nil {
			return
		}

		if !u.IsAbs() {
			if base == nil {
				return
			}
			u = base.ResolveReference(u)
		}

		s := u.String()
		if s == "" {
			return
		}

		lower := strings.ToLower(s)
		if strings.Contains(lower, "/detail/") {
			return
		}
		if strings.HasSuffix(lower, ".css") ||
			strings.HasSuffix(lower, ".js") ||
			strings.HasSuffix(lower, ".png") ||
			strings.HasSuffix(lower, ".jpg") ||
			strings.HasSuffix(lower, ".jpeg") ||
			strings.HasSuffix(lower, ".svg") ||
			strings.HasSuffix(lower, ".ico") ||
			strings.Contains(lower, "favicon") {
			return
		}

		// 优先保留疑似跳转 URL；否则保留同域且包含参数的 URL 作为兜底。
		isLikelyJump := strings.Contains(lower, "/go/") ||
			strings.Contains(lower, "/jump") ||
			strings.Contains(lower, "/redirect") ||
			strings.Contains(lower, "/api/") ||
			strings.Contains(lower, "/ajax/") ||
			strings.Contains(lower, "transfer") ||
			strings.Contains(lower, "url=") ||
			strings.Contains(lower, "target=") ||
			strings.Contains(lower, "agree") ||
			strings.Contains(lower, "consent") ||
			strings.Contains(lower, "token=") ||
			strings.Contains(lower, "id=")
		if !isLikelyJump {
			if base == nil || u.Host != base.Host || u.RawQuery == "" {
				return
			}
		}

		if _, ok := seen[s]; ok {
			return
		}
		if len(candidates) >= 30 {
			return
		}
		seen[s] = struct{}{}
		candidates = append(candidates, s)
	}

	attrNames := []string{"href", "data-href", "data-url", "data-link", "value", "onclick"}
	doc.Find("*").Each(func(i int, s *goquery.Selection) {
		// 先遍历节点的全部属性，避免漏掉非常见 data-* 字段。
		if len(s.Nodes) > 0 {
			for _, attr := range s.Nodes[0].Attr {
				if attr.Val != "" {
					addCandidate(attr.Val)
				}
			}
		}

		for _, attr := range attrNames {
			if v, ok := s.Attr(attr); ok && v != "" {
				addCandidate(v)
			}
		}
	})

	doc.Find("script").Each(func(i int, s *goquery.Selection) {
		scriptText := strings.TrimSpace(s.Text())
		if scriptText == "" {
			return
		}
		for _, u := range p.extractAllURLs(scriptText) {
			addCandidate(u)
		}
		for _, m := range escapedHTTPURLRegex.FindAllString(scriptText, -1) {
			addCandidate(m)
		}
		for _, match := range quotedPathRegex.FindAllStringSubmatch(scriptText, -1) {
			if len(match) > 1 {
				addCandidate(match[1])
			}
		}
		for _, match := range locationAssignRegex.FindAllStringSubmatch(scriptText, -1) {
			if len(match) > 1 {
				addCandidate(match[1])
			}
		}
		for _, match := range jsInvokeRegex.FindAllStringSubmatch(scriptText, -1) {
			if len(match) > 2 {
				fn := strings.ToLower(match[1])
				args := strings.ToLower(match[2])
				if strings.Contains(fn, "go") || strings.Contains(fn, "jump") || strings.Contains(fn, "link") ||
					strings.Contains(fn, "get") || strings.Contains(fn, "open") || strings.Contains(fn, "share") ||
					strings.Contains(args, "id") || strings.Contains(args, "agree") {
					addCandidate("/api/" + match[1])
					addCandidate("/" + match[1])
				}
			}
		}
	})

	return candidates
}

// extractAllURLs 用正则批量提取文本中的所有网盘链接候选。
func (p *PiozAsyncPlugin) extractAllURLs(text string) []string {
	if text == "" {
		return nil
	}

	var urls []string
	seen := make(map[string]struct{}, 16)

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

	variants := []string{
		text,
		p.normalizeEscapedTextForURLExtraction(text),
	}

	for _, variant := range variants {
		if variant == "" {
			continue
		}

		for _, regex := range patterns {
			matches := regex.FindAllString(variant, -1)
			for _, m := range matches {
				m = strings.TrimSpace(m)
				if m == "" {
					continue
				}
				if _, ok := seen[m]; ok {
					continue
				}
				seen[m] = struct{}{}
				urls = append(urls, m)
			}
		}

		// Catch escaped http URLs from inline JSON/JS, then normalize and re-check.
		for _, escaped := range escapedHTTPURLRegex.FindAllString(variant, -1) {
			normalized := p.normalizeEscapedTextForURLExtraction(escaped)
			if normalized == "" || !p.isValidNetworkDriveURL(normalized) {
				continue
			}
			if _, ok := seen[normalized]; ok {
				continue
			}
			seen[normalized] = struct{}{}
			urls = append(urls, normalized)
		}

		// Some pages embed quark domain without scheme: pan.quark.cn/s/xxxx
		for _, m := range quarkLinkNoSchemeRegex.FindAllString(variant, -1) {
			u := "https://" + strings.TrimSpace(m)
			if u == "" || !p.isValidNetworkDriveURL(u) {
				continue
			}
			if _, ok := seen[u]; ok {
				continue
			}
			seen[u] = struct{}{}
			urls = append(urls, u)
		}
	}

	return urls
}

// normalizeEscapedTextForURLExtraction 归一化脚本中的转义 URL 形式。
func (p *PiozAsyncPlugin) normalizeEscapedTextForURLExtraction(text string) string {
	s := strings.TrimSpace(text)
	if s == "" {
		return ""
	}

	// 常见 JS/JSON 转义
	s = strings.ReplaceAll(s, `\\u002F`, `/`)
	s = strings.ReplaceAll(s, `\u002F`, `/`)
	s = strings.ReplaceAll(s, `\\u003A`, `:`)
	s = strings.ReplaceAll(s, `\u003A`, `:`)
	s = strings.ReplaceAll(s, `\\u0026`, `&`)
	s = strings.ReplaceAll(s, `\u0026`, `&`)
	s = strings.ReplaceAll(s, `\\u003D`, `=`)
	s = strings.ReplaceAll(s, `\u003D`, `=`)
	s = strings.ReplaceAll(s, `\\\/`, `/`)
	s = strings.ReplaceAll(s, `\/`, `/`)
	s = strings.ReplaceAll(s, `\\`, `\`)

	// 清理包裹符号和常见尾随标点
	s = strings.Trim(s, `"'()[]{}<>`)
	s = strings.TrimRight(s, ".,;")

	return s
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
		"search_requests":      totalSearchRequests,
		"detail_requests":      totalDetailRequests,
		"cache_hits":           totalCacheHits,
		"cache_misses":         totalCacheMisses,
		"cache_hit_rate":       cacheHitRate,
		"anti_crawler_blocks":  totalAntiCrawlerBlocks,
		"block_rate":           blockRate,
		"avg_search_time_ms":   avgSearchTime,
		"avg_detail_time_ms":   avgDetailTime,
		"total_search_time_ns": totalSearchTime,
		"total_detail_time_ns": totalDetailTime,
		"session_cookies":      len(sessionCookies),
		"current_user_agent":   p.currentUserAgent,
	}
}

type cachedResponse struct {
	results   []model.SearchResult
	timestamp time.Time
}
