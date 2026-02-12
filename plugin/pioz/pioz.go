package pioz

import (
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

const (
	// 超时时间配置
	DefaultTimeout = 15 * time.Second  // 搜索超时
	DetailTimeout  = 12 * time.Second  // 详情页超时
	APIBaseURL     = "https://www.pioz.cn/api"
	SiteBaseURL    = "https://www.pioz.cn"
	
	// 并发控制 - 平衡性能和反爬
	MaxConcurrency = 8
	
	// HTTP连接池配置
	MaxIdleConns        = 50
	MaxIdleConnsPerHost = 10
	MaxConnsPerHost     = 20
	IdleConnTimeout     = 60 * time.Second
	
	// 缓存配置
	CacheTTL = 30 * time.Minute
	
	// 反爬策略配置
	RequestDelayMin = 500 * time.Millisecond  // 最小请求间隔
	RequestDelayMax = 1500 * time.Millisecond // 最大请求间隔
	RetryCount      = 2                       // 重试次数
)

func init() {
	plugin.RegisterGlobalPlugin(NewPiozPlugin())
}

// 预编译的正则表达式（支持16种网盘链接）
var (
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
	
	// 密码提取正则
	passwordRegex    = regexp.MustCompile(`(?i)(?:提取码|密码|pwd|码)[：:]\s*([a-zA-Z0-9]{4})`)
	urlPasswordRegex = regexp.MustCompile(`(?i)\?pwd=([0-9a-zA-Z]+)`)
	
	// 详情页ID提取
	detailIDRegex = regexp.MustCompile(`/detail/(\d+)`)
	
	// 反爬检测正则
	antiCrawlerRegex = regexp.MustCompile(`禁止使用开发者工具|偷样式死全家|反爬虫|防爬`)
)

// 缓存和会话管理
var (
	searchCache     = sync.Map{}
	detailCache     = sync.Map{}
	transferCache   = sync.Map{}
	sessionCookies  []*http.Cookie
	sessionMutex    sync.RWMutex
	lastRequestTime time.Time
	requestCounter  int64
)

// 性能统计
var (
	searchRequests    int64 = 0
	detailRequests    int64 = 0
	cacheHits         int64 = 0
	cacheMisses       int64 = 0
	antiCrawlerBlocks int64 = 0
	totalSearchTime   int64 = 0
	totalDetailTime   int64 = 0
)

// 缓存响应结构
type cachedResponse struct {
	results   []model.SearchResult
	timestamp time.Time
}

// DeepSearchResponse API响应结构
type DeepSearchResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Total   int    `json:"total"`
	Results []struct {
		ID         string `json:"id"`
		Title      string `json:"title"`
		CloudType  string `json:"cloud_type"`
		Datetime   string `json:"datetime"`
		Size       string `json:"size"`
		Desc       string `json:"desc"`
		CreateTime string `json:"create_time"`
		ViewURL    string `json:"view_url"`
	} `json:"results"`
}

// PiozPlugin Pioz 插件结构
type PiozPlugin struct {
	*plugin.BaseAsyncPlugin
	optimizedClient  *http.Client
	userAgents       []string
	currentUserAgent string
	// ✅ 添加对象池减少内存分配
	linkPool sync.Pool
}

// init 注册插件
func init() {
	plugin.RegisterGlobalPlugin(NewPiozPlugin())
}

// NewPiozPlugin 创建新的 Pioz 插件实例
func NewPiozPlugin() *PiozPlugin {
	// 创建优化的 HTTP 客户端
	transport := &http.Transport{
		MaxIdleConns:        MaxIdleConns,
		MaxIdleConnsPerHost: MaxIdleConnsPerHost,
		MaxConnsPerHost:     MaxConnsPerHost,
		IdleConnTimeout:     IdleConnTimeout,
		DisableKeepAlives:   false,
	}
	
	client := &http.Client{
		Transport: transport,
		Timeout:   DefaultTimeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			// ✅ 限制重定向次数
			if len(via) >= 10 {
				return http.ErrUseLastResponse
			}
			return nil
		},
	}
	
	// 多个 User-Agent 用于随机切换
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
	
	p := &PiozPlugin{
		BaseAsyncPlugin:  plugin.NewBaseAsyncPlugin("pioz", 1),
		optimizedClient:  client,
		userAgents:       userAgents,
		currentUserAgent: userAgents[randomIndex],
	}
	
	// ✅ 初始化对象池
	p.linkPool = sync.Pool{
		New: func() interface{} {
			return &model.Link{}
		},
	}
	
	return p
}

// Search 同步搜索接口
func (p *PiozPlugin) Search(keyword string, ext map[string]interface{}) ([]model.SearchResult, error) {
	result, err := p.SearchWithResult(keyword, ext)
	if err != nil {
		return nil, err
	}
	return result.Results, nil
}

// SearchWithResult 带结果统计的搜索接口
func (p *PiozPlugin) SearchWithResult(keyword string, ext map[string]interface{}) (model.PluginSearchResult, error) {
	return p.AsyncSearchWithResult(keyword, p.searchImpl, p.MainCacheKey, ext)
}

// searchImpl 实现搜索逻辑（多策略搜索）
func (p *PiozPlugin) searchImpl(client *http.Client, keyword string, ext map[string]interface{}) ([]model.SearchResult, error) {
	fmt.Printf("[%s] 开始搜索，keyword='%s'\n", p.Name(), keyword)
	
	// 性能统计
	start := time.Now()
	atomic.AddInt64(&searchRequests, 1)
	defer func() {
		duration := time.Since(start).Nanoseconds()
		atomic.AddInt64(&totalSearchTime, duration)
	}()
	
	// 检查缓存
	cacheKey := fmt.Sprintf("%s:%s", p.Name(), keyword)
	if cached, ok := searchCache.Load(cacheKey); ok {
		if cachedResp, ok := cached.(cachedResponse); ok {
			if time.Since(cachedResp.timestamp) < CacheTTL {
				atomic.AddInt64(&cacheHits, 1)
				fmt.Printf("[%s] 命中缓存，结果数: %d\n", p.Name(), len(cachedResp.results))
				return cachedResp.results, nil
			}
		}
	}
	atomic.AddInt64(&cacheMisses, 1)
	
	// 使用优化的客户端
	if p.optimizedClient != nil {
		client = p.optimizedClient
	}
	
	// 应用反爬延迟
	p.applyAntiCrawlerDelay()
	
	// 策略1：深度搜索 API（首选）
	results, err := p.performDeepSearch(client, keyword)
	if err == nil && len(results) > 0 {
		fmt.Printf("[%s] ✅ 策略1成功：深度搜索API返回 %d 个结果\n", p.Name(), len(results))
		enhancedResults := p.enhanceWithDetails(client, results)
		searchCache.Store(cacheKey, cachedResponse{
			results:   enhancedResults,
			timestamp: time.Now(),
		})
		return enhancedResults, nil
	}
	fmt.Printf("[%s] ⚠️ 策略1失败：%v\n", p.Name(), err)
	
	// 策略2：普通搜索页面（备用）
	p.applyAntiCrawlerDelay()
	fmt.Printf("[%s] 尝试策略2：普通HTML搜索\n", p.Name())
	results, err = p.performRegularSearch(client, keyword)
	if err == nil && len(results) > 0 {
		fmt.Printf("[%s] ✅ 策略2成功：HTML搜索返回 %d 个结果\n", p.Name(), len(results))
		enhancedResults := p.enhanceWithDetails(client, results)
		searchCache.Store(cacheKey, cachedResponse{
			results:   enhancedResults,
			timestamp: time.Now(),
		})
		return enhancedResults, nil
	}
	fmt.Printf("[%s] ⚠️ 策略2失败：%v\n", p.Name(), err)
	
	// 策略3：首页热搜榜匹配（最后手段）
	p.applyAntiCrawlerDelay()
	fmt.Printf("[%s] 尝试策略3：热搜榜匹配\n", p.Name())
	results, err = p.extractFromHotSearch(client, keyword)
	if err != nil {
		fmt.Printf("[%s] ❌ 策略3失败：%v\n", p.Name(), err)
		return nil, fmt.Errorf("[%s] 所有搜索策略都失败: %w", p.Name(), err)
	}
	fmt.Printf("[%s] ✅ 策略3成功：热搜榜返回 %d 个结果\n", p.Name(), len(results))
	
	enhancedResults := p.enhanceWithDetails(client, results)
	searchCache.Store(cacheKey, cachedResponse{
		results:   enhancedResults,
		timestamp: time.Now(),
	})
	
	return enhancedResults, nil
}

// performDeepSearch 执行深度搜索API
func (p *PiozPlugin) performDeepSearch(client *http.Client, keyword string) ([]model.SearchResult, error) {
	// 构建API URL
	apiURL := fmt.Sprintf("%s/deep-search?kw=%s", APIBaseURL, url.QueryEscape(keyword))
	fmt.Printf("[%s] 调用API: %s\n", p.Name(), apiURL)
	
	// 创建请求
	ctx, cancel := context.WithTimeout(context.Background(), DefaultTimeout)
	defer cancel()
	
	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}
	
	// 设置API请求头
	p.setAPIHeaders(req)
	p.addSessionCookies(req)
	
	// 发送请求（带重试）
	resp, err := p.doRequestWithRetry(client, req)
	if err != nil {
		return nil, fmt.Errorf("请求失败: %w", err)
	}
	defer resp.Body.Close()
	
	fmt.Printf("[%s] API响应状态码: %d\n", p.Name(), resp.StatusCode)
	
	// 检查反爬
	if p.checkAntiCrawlerResponse(resp) {
		atomic.AddInt64(&antiCrawlerBlocks, 1)
		return nil, fmt.Errorf("触发反爬保护")
	}
	
	if resp.StatusCode != 200 {
		// ⚠️ API可能已失效，记录详细信息后降级到其他策略
		fmt.Printf("[%s] ⚠️ API返回状态码: %d，将尝试其他搜索策略\n", p.Name(), resp.StatusCode)
		return nil, fmt.Errorf("API返回状态码: %d", resp.StatusCode)
	}
	
	// 读取响应
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %w", err)
	}
	
	fmt.Printf("[%s] 响应长度: %d 字节\n", p.Name(), len(body))
	
	// 检查响应是否包含反爬内容
	if antiCrawlerRegex.Match(body) {
		atomic.AddInt64(&antiCrawlerBlocks, 1)
		return nil, fmt.Errorf("响应包含反爬内容")
	}
	
	// 解析JSON
	var apiResp DeepSearchResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return nil, fmt.Errorf("解析JSON失败: %w", err)
	}
	
	fmt.Printf("[%s] API返回: code=%d, total=%d, results=%d\n", 
		p.Name(), apiResp.Code, apiResp.Total, len(apiResp.Results))
	
	if apiResp.Code != 0 {
		return nil, fmt.Errorf("API错误: %s", apiResp.Message)
	}
	
	if len(apiResp.Results) == 0 {
		return nil, fmt.Errorf("未找到搜索结果")
	}
	
	// 转换为 SearchResult
	var results []model.SearchResult
	for _, item := range apiResp.Results {
		result := p.convertToSearchResult(item)
		results = append(results, result)
	}
	
	fmt.Printf("[%s] 深度搜索找到 %d 个结果\n", p.Name(), len(results))
	return results, nil
}

// convertToSearchResult 将API结果转换为SearchResult
func (p *PiozPlugin) convertToSearchResult(item struct {
	ID         string `json:"id"`
	Title      string `json:"title"`
	CloudType  string `json:"cloud_type"`
	Datetime   string `json:"datetime"`
	Size       string `json:"size"`
	Desc       string `json:"desc"`
	CreateTime string `json:"create_time"`
	ViewURL    string `json:"view_url"`
}) model.SearchResult {
	// 构建内容描述
	var contentParts []string
	
	// 云盘类型
	if item.CloudType != "" {
		cloudTypeName := p.getCloudTypeName(item.CloudType)
		if cloudTypeName != "" {
			contentParts = append(contentParts, "类型: "+cloudTypeName)
		}
	}
	
	// 大小
	if item.Size != "" {
		contentParts = append(contentParts, "大小: "+item.Size)
	}
	
	// 时间
	if item.Datetime != "" && item.Datetime != "0001-01-01T00:00:00Z" {
		contentParts = append(contentParts, "分享时间: "+item.Datetime)
	}
	
	// 构建详情页URL
	viewURL := item.ViewURL
	if viewURL == "" {
		viewURL = fmt.Sprintf("%s/detail/%s", SiteBaseURL, item.ID)
	}
	
	// 解析时间
	datetime := p.parseTime(item.Datetime)
	
	// ✅ 修复：包含 URL，便于详情页解析
	uniqueID := fmt.Sprintf("%s-%s-%s", p.Name(), item.ID, url.QueryEscape(viewURL))
	
	return model.SearchResult{
		MessageID: uniqueID,
		UniqueID:  uniqueID,  // ✅ 使用包含URL的格式
		Title:     item.Title,
		Content:   strings.Join(contentParts, " | "),
		Datetime:  datetime,
		Links:     []model.Link{},  // ❌ 改为空，等待 enhanceWithDetails 填充
		Channel:   "",              // ⭐ 重要：插件搜索结果Channel必须为空
	}
}

// parseTime 解析时间字符串
func (p *PiozPlugin) parseTime(timeStr string) time.Time {
	if timeStr == "" || timeStr == "0001-01-01T00:00:00Z" {
		return time.Now()
	}
	
	// 尝试多种时间格式
	formats := []string{
		time.RFC3339,
		"2006-01-02T15:04:05Z",
		"2006-01-02 15:04:05",
		"2006-01-02",
		"2006-1-2",
	}
	
	for _, format := range formats {
		if t, err := time.Parse(format, timeStr); err == nil {
			return t
		}
	}
	
	// 如果解析失败，返回当前时间
	return time.Now()
}


// performRegularSearch 执行普通搜索（HTML页面）
func (p *PiozPlugin) performRegularSearch(client *http.Client, keyword string) ([]model.SearchResult, error) {
	searchURL := fmt.Sprintf("%s/search?q=%s", SiteBaseURL, url.QueryEscape(keyword))
	fmt.Printf("[%s] HTML搜索URL: %s\n", p.Name(), searchURL)
	
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
	
	fmt.Printf("[%s] HTML搜索响应状态码: %d\n", p.Name(), resp.StatusCode)
	
	if p.checkAntiCrawlerResponse(resp) {
		atomic.AddInt64(&antiCrawlerBlocks, 1)
		return nil, fmt.Errorf("触发反爬保护")
	}
	
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("搜索页面返回状态码: %d", resp.StatusCode)
	}
	
	doc, err := goquery.NewDocumentFromReader(resp.Body)
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
		return nil, fmt.Errorf("未找到搜索结果")
	}
	
	fmt.Printf("[%s] 普通搜索找到 %d 个结果\n", p.Name(), len(results))
	return results, nil
}

// extractFromHotSearch 从首页热搜榜提取
func (p *PiozPlugin) extractFromHotSearch(client *http.Client, keyword string) ([]model.SearchResult, error) {
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
		return nil, fmt.Errorf("首页访问失败: %d", resp.StatusCode)
	}
	
	doc, err := goquery.NewDocumentFromReader(resp.Body)
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
		if title == "" || !strings.Contains(strings.ToLower(title), keywordLower) {
			return
		}
		
		href, exists := s.Attr("href")
		if !exists {
			return
		}
		
		matches := detailIDRegex.FindStringSubmatch(href)
		if len(matches) < 2 {
			return
		}
		
		detailID := matches[1]
		detailURL := fmt.Sprintf("%s/detail/%s", SiteBaseURL, detailID)
		
		result := model.SearchResult{
			UniqueID: fmt.Sprintf("%s-%s-%s", p.Name(), detailID, url.QueryEscape(detailURL)),
			Title:    title,
			Content:  "来自热搜榜的推荐资源 | 来源: hot_search",
			Links:    []model.Link{},
			Channel:  "",
			Datetime: time.Time{},
		}
		results = append(results, result)
	})
	
	if len(results) == 0 {
		return nil, fmt.Errorf("热搜榜未找到匹配结果")
	}
	
	fmt.Printf("[%s] 热搜榜找到 %d 个匹配结果\n", p.Name(), len(results))
	return results, nil
}

// parseSearchItem 解析单个搜索结果项
func (p *PiozPlugin) parseSearchItem(s *goquery.Selection, keyword string, index int) model.SearchResult {
	result := model.SearchResult{}
	
	// 提取标题
	title := ""
	titleSelectors := []string{".text-gray-100", "[class*='title']", ".hot-search-title-text", "span[title]"}
	
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
		return result
	}
	
	// 提取详情页链接
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
	
	// 构建唯一ID
	if detailURL != "" {
		result.UniqueID = fmt.Sprintf("%s-detail-%s", p.Name(), url.QueryEscape(detailURL))
	} else {
		result.UniqueID = fmt.Sprintf("%s-html-%d-%d", p.Name(), index, time.Now().UnixNano())
	}
	
	result.Title = title
	result.Content = "来源: html_search"
	result.Datetime = time.Time{}
	result.Links = []model.Link{}
	result.Channel = ""
	
	return result
}

// parseDetailLink 解析详情页链接
func (p *PiozPlugin) parseDetailLink(a *goquery.Selection, index int) model.SearchResult {
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
		title = "资源详情"
	}
	
	var detailURL string
	if strings.HasPrefix(href, "http") {
		detailURL = href
	} else {
		detailURL = SiteBaseURL + href
	}
	
	result.UniqueID = fmt.Sprintf("%s-%s-%s", p.Name(), detailID, url.QueryEscape(detailURL))
	result.Title = title
	result.Content = "来自搜索结果页 | 来源: html_search"
	result.Links = []model.Link{}
	result.Channel = ""
	result.Datetime = time.Time{}
	
	return result
}

// enhanceWithDetails 异步获取详情页信息（二次跳转）
func (p *PiozPlugin) enhanceWithDetails(client *http.Client, results []model.SearchResult) []model.SearchResult {
	if len(results) == 0 {
		return results
	}
	
	var enhancedResults []model.SearchResult
	var mu sync.Mutex
	var wg sync.WaitGroup
	
	// ✅ 根据结果数量动态调整并发度
	concurrency := MaxConcurrency
	if len(results) < 5 {
		concurrency = 2 // 结果少时降低并发
	} else if len(results) > 20 {
		concurrency = 12 // 结果多时适当提高并发
	}
	
	semaphore := make(chan struct{}, concurrency)
	
	fmt.Printf("[%s] 开始增强 %d 个结果，并发度: %d\n", p.Name(), len(results), concurrency)
	
	for _, result := range results {
		wg.Add(1)
		
		go func(r model.SearchResult) {
			defer wg.Done()
			
			semaphore <- struct{}{}
			defer func() { <-semaphore }()
			
			// 应用反爬延迟
			p.applyAntiCrawlerDelay()
			
			// 检查缓存
			if cached, ok := detailCache.Load(r.UniqueID); ok {
				if cachedResult, ok := cached.(model.SearchResult); ok {
					mu.Lock()
					enhancedResults = append(enhancedResults, cachedResult)
					mu.Unlock()
					return
				}
			}
			
			// 获取详情信息
			links := p.fetchResourceInfo(client, r)
			r.Links = links
			
			// 如果有链接，记录日志
			if len(links) > 0 {
				fmt.Printf("[%s] 成功获取资源链接: %s -> %d个链接\n", 
					p.Name(), r.Title, len(links))
			}
			
			// 缓存结果
			detailCache.Store(r.UniqueID, r)
			
			mu.Lock()
			enhancedResults = append(enhancedResults, r)
			mu.Unlock()
		}(result)
	}
	
	wg.Wait()
	fmt.Printf("[%s] 增强完成，成功: %d/%d\n", p.Name(), len(enhancedResults), len(results))
	return enhancedResults
}

// fetchResourceInfo 获取资源信息
func (p *PiozPlugin) fetchResourceInfo(client *http.Client, result model.SearchResult) []model.Link {
	// 性能统计
	start := time.Now()
	atomic.AddInt64(&detailRequests, 1)
	defer func() {
		duration := time.Since(start).Nanoseconds()
		atomic.AddInt64(&totalDetailTime, duration)
	}()
	
	// ✅ 添加调试日志
	fmt.Printf("[%s] 开始获取详情: UniqueID=%s, Title=%s\n", p.Name(), result.UniqueID, result.Title)
	
	// 方法1：尝试transfer API（首选）
	links := p.tryTransferAPI(client, result)
	if len(links) > 0 {
		fmt.Printf("[%s] Transfer API 成功: %d个链接\n", p.Name(), len(links))
		return links
	}
	
	// 方法2：解析详情页HTML
	fmt.Printf("[%s] Transfer API 失败，尝试解析详情页\n", p.Name())
	links = p.parseResourceDetailPage(client, result)
	if len(links) > 0 {
		fmt.Printf("[%s] 详情页解析成功: %d个链接\n", p.Name(), len(links))
	} else {
		fmt.Printf("[%s] 详情页解析失败，未找到链接\n", p.Name())
	}
	
	return links
}

// tryTransferAPI 尝试transfer API
func (p *PiozPlugin) tryTransferAPI(client *http.Client, result model.SearchResult) []model.Link {
	var resourceID string
	
	// 方案1：从 UniqueID 提取
	parts := strings.Split(result.UniqueID, "-")
	if len(parts) >= 2 {
		resourceID = parts[1]
	}
	
	// ✅ 方案2：从 MessageID 提取（pioz 特有）
	if resourceID == "" && result.MessageID != "" {
		msgParts := strings.Split(result.MessageID, "-")
		if len(msgParts) >= 2 {
			resourceID = msgParts[1]
		}
	}
	
	// ✅ 方案3：从详情页链接中提取
	if resourceID == "" {
		for _, link := range result.Links {
			if link.Type == "detail" {
				matches := detailIDRegex.FindStringSubmatch(link.URL)
				if len(matches) > 1 {
					resourceID = matches[1]
					break
				}
			}
		}
	}
	
	if resourceID == "" {
		fmt.Printf("[%s] 无法提取资源ID: UniqueID=%s\n", p.Name(), result.UniqueID)
		return nil
	}
	
	// ✅ 优化缓存键，添加插件名前缀
	cacheKey := fmt.Sprintf("pioz:transfer:%s", resourceID)
	if cached, ok := transferCache.Load(cacheKey); ok {
		if links, ok := cached.([]model.Link); ok && len(links) > 0 {
			fmt.Printf("[%s] Transfer缓存命中: resourceID=%s\n", p.Name(), resourceID)
			return links
		}
	}
	
	// 调用transfer API
	transferURL := fmt.Sprintf("%s/transfer?id=%s", APIBaseURL, url.QueryEscape(resourceID))
	fmt.Printf("[%s] 调用Transfer API: resourceID=%s\n", p.Name(), resourceID)
	
	// ✅ 添加超时控制
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	
	req, err := http.NewRequestWithContext(ctx, "GET", transferURL, nil)
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
	
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil
	}
	
	var transferResp struct {
		Success bool `json:"success"`
		Data    struct {
			URL      string `json:"url"`
			Password string `json:"password"`
			Type     string `json:"type"`
		} `json:"data"`
	}
	
	if err := json.Unmarshal(body, &transferResp); err != nil {
		return nil
	}
	
	if !transferResp.Success || transferResp.Data.URL == "" {
		return nil
	}
	
	// 解析链接类型和密码
	link := p.createLinkFromURL(transferResp.Data.URL, transferResp.Data.Password)
	links := []model.Link{link}
	
	// 缓存结果
	transferCache.Store(cacheKey, links)
	
	return links
}

// parseResourceDetailPage 解析资源详情页
func (p *PiozPlugin) parseResourceDetailPage(client *http.Client, result model.SearchResult) []model.Link {
	// 从UniqueID提取详情页URL
	detailURL := ""
	
	// 方案1：从 UniqueID 提取（依赖正确的格式）
	parts := strings.Split(result.UniqueID, "-")
	if len(parts) >= 3 {
		// 第三部分是编码的URL
		var err error
		detailURL, err = url.QueryUnescape(parts[2])
		if err != nil {
			detailURL = parts[2]
		}
	}
	
	// ✅ 方案2：直接从已有的详情页链接提取（兜底）
	if detailURL == "" && len(result.Links) > 0 {
		for _, link := range result.Links {
			if link.Type == "detail" {
				detailURL = link.URL
				break
			}
		}
	}
	
	// 方案3：从 Content 中提取详情页URL（最后手段）
	if detailURL == "" && strings.Contains(result.Content, "detail/") {
		// 尝试从 Content 中提取详情页URL
		if idx := strings.Index(result.Content, "https://www.pioz.cn/detail/"); idx != -1 {
			endIdx := strings.IndexAny(result.Content[idx:], " |")
			if endIdx != -1 {
				detailURL = result.Content[idx : idx+endIdx]
			} else {
				detailURL = result.Content[idx:]
			}
		}
	}
	
	if detailURL == "" {
		fmt.Printf("[%s] 无法提取详情页URL: UniqueID=%s\n", p.Name(), result.UniqueID)
		return nil
	}
	
	fmt.Printf("[%s] 解析详情页: %s\n", p.Name(), detailURL)
	
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
	
	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil
	}
	
	// 提取链接
	return p.extractLinksFromDocument(doc)
}

// extractLinksFromDocument 从文档中提取链接
func (p *PiozPlugin) extractLinksFromDocument(doc *goquery.Document) []model.Link {
	var links []model.Link
	pageText := doc.Text()
	
	// 提取所有网盘链接
	urls := p.extractAllURLs(pageText)
	
	// 从链接元素中提取
	doc.Find("a[href]").Each(func(i int, s *goquery.Selection) {
		href, exists := s.Attr("href")
		if exists && p.isValidNetworkDriveURL(href) {
			urls = append(urls, href)
		}
	})
	
	for _, urlStr := range urls {
		linkType := p.determineLinkType(urlStr)
		if linkType == "" || linkType == "unknown" {
			continue
		}
		
		// 提取密码
		password := p.extractPasswordFromURL(urlStr)
		if password == "" {
			password = p.extractPasswordFromText(pageText)
		}
		
		link := model.Link{
			Type:     linkType,
			URL:      urlStr,
			Password: password,
		}
		
		// 避免重复
		if !p.containsLink(links, link) {
			links = append(links, link)
		}
	}
	
	return links
}

// extractAllURLs 从文本中提取所有URL
func (p *PiozPlugin) extractAllURLs(text string) []string {
	var urls []string
	
	// 使用正则表达式匹配所有支持的网盘链接
	patterns := []*regexp.Regexp{
		quarkLinkRegex, baiduLinkRegex, aliyunLinkRegex, ucLinkRegex,
		xunleiLinkRegex, tianyiLinkRegex, lanzouLinkRegex, link115Regex,
		mobileLinkRegex, weiyunLinkRegex, jianguoyunLinkRegex, link123Regex,
		pikpakLinkRegex, magnetLinkRegex, ed2kLinkRegex,
	}
	
	for _, regex := range patterns {
		matches := regex.FindAllString(text, -1)
		urls = append(urls, matches...)
	}
	
	return urls
}

// 反爬绕过策略

// applyAntiCrawlerDelay 应用反爬延迟
func (p *PiozPlugin) applyAntiCrawlerDelay() {
	now := time.Now()
	
	// 计算距离上次请求的时间
	timeSinceLast := now.Sub(lastRequestTime)
	
	// 如果请求间隔太短，添加延迟
	if timeSinceLast < RequestDelayMin {
		delay := RequestDelayMin - timeSinceLast
		// 添加随机延迟，避免固定模式
		randomDelay := time.Duration(time.Now().UnixNano()%500) * time.Millisecond
		totalDelay := delay + randomDelay
		
		if totalDelay > 0 {
			time.Sleep(totalDelay)
		}
	}
	
	// 更新最后请求时间
	lastRequestTime = time.Now()
	
	// 每5次请求随机切换User-Agent
	requestCount := atomic.AddInt64(&requestCounter, 1)
	if requestCount%5 == 0 {
		randomIndex := time.Now().UnixNano() % int64(len(p.userAgents))
		p.currentUserAgent = p.userAgents[randomIndex]
	}
}

// setStealthHeaders 设置隐身请求头
func (p *PiozPlugin) setStealthHeaders(req *http.Request) {
	req.Header.Set("User-Agent", p.currentUserAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.7")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8,en-GB;q=0.7,en-US;q=0.6")
	req.Header.Set("Accept-Encoding", "gzip, deflate, br")
	req.Header.Set("Connection", "keep-alive")
	req.Header.Set("Upgrade-Insecure-Requests", "1")
	req.Header.Set("Sec-Fetch-Dest", "document")
	req.Header.Set("Sec-Fetch-Mode", "navigate")
	req.Header.Set("Sec-Fetch-Site", "same-origin")
	req.Header.Set("Sec-Fetch-User", "?1")
	req.Header.Set("Cache-Control", "max-age=0")
	req.Header.Set("Pragma", "no-cache")
	
	// ✅ 添加现代浏览器安全头
	req.Header.Set("Sec-Ch-Ua", `"Not_A Brand";v="8", "Chromium";v="120", "Microsoft Edge";v="120"`)
	req.Header.Set("Sec-Ch-Ua-Mobile", "?0")
	req.Header.Set("Sec-Ch-Ua-Platform", `"Windows"`)
	
	if req.URL.Host == "www.pioz.cn" {
		req.Header.Set("Referer", "https://www.pioz.cn/")
	}
}

// setAPIHeaders 设置API请求头
func (p *PiozPlugin) setAPIHeaders(req *http.Request) {
	req.Header.Set("User-Agent", p.currentUserAgent)
	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")
	req.Header.Set("Accept-Encoding", "gzip, deflate, br")
	req.Header.Set("Connection", "keep-alive")
	req.Header.Set("X-Requested-With", "XMLHttpRequest")
	req.Header.Set("Sec-Fetch-Dest", "empty")
	req.Header.Set("Sec-Fetch-Mode", "cors")
	req.Header.Set("Sec-Fetch-Site", "same-origin")
	req.Header.Set("Referer", "https://www.pioz.cn/")
	req.Header.Set("Origin", "https://www.pioz.cn")
}

// addSessionCookies 添加会话cookies
func (p *PiozPlugin) addSessionCookies(req *http.Request) {
	sessionMutex.RLock()
	defer sessionMutex.RUnlock()
	
	for _, cookie := range sessionCookies {
		req.AddCookie(cookie)
	}
	
	// 如果没有cookies，添加一些默认的
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

// checkAntiCrawlerResponse 检查反爬响应
func (p *PiozPlugin) checkAntiCrawlerResponse(resp *http.Response) bool {
	// 检查状态码
	if resp.StatusCode == 403 || resp.StatusCode == 429 {
		return true
	}
	
	// 检查响应头
	if strings.Contains(resp.Header.Get("Server"), "anti-crawler") {
		return true
	}
	
	return false
}

// doRequestWithRetry 带重试机制的HTTP请求
func (p *PiozPlugin) doRequestWithRetry(client *http.Client, req *http.Request) (*http.Response, error) {
	var lastErr error
	
	// ✅ 添加最大重试时间限制
	maxRetryTime := 5 * time.Second
	startTime := time.Now()
	
	for i := 0; i < RetryCount; i++ {
		if time.Since(startTime) > maxRetryTime {
			return nil, fmt.Errorf("重试超时")
		}
		
		if i > 0 {
			// 指数退避
			backoff := time.Duration(1<<uint(i-1)) * 500 * time.Millisecond
			time.Sleep(backoff)
			
			// 随机切换User-Agent
			randomIndex := time.Now().UnixNano() % int64(len(p.userAgents))
			req.Header.Set("User-Agent", p.userAgents[randomIndex])
		}
		
		resp, err := client.Do(req)
		if err == nil {
			// 更新cookies
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
	
	return nil, fmt.Errorf("重试 %d 次后失败: %w", RetryCount, lastErr)
}

// 辅助函数

// isValidNetworkDriveURL 检查是否为有效的网盘URL
func (p *PiozPlugin) isValidNetworkDriveURL(urlStr string) bool {
	if strings.Contains(urlStr, "javascript:") || 
	   strings.Contains(urlStr, "#") ||
	   urlStr == "" ||
	   (!strings.HasPrefix(urlStr, "http") && !strings.HasPrefix(urlStr, "magnet:") && !strings.HasPrefix(urlStr, "ed2k:")) {
		return false
	}
	
	linkType := p.determineLinkType(urlStr)
	return linkType != "" && linkType != "unknown"
}

// determineLinkType 判断链接类型（支持16种）
func (p *PiozPlugin) determineLinkType(urlStr string) string {
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

// extractPasswordFromURL 从URL提取密码
func (p *PiozPlugin) extractPasswordFromURL(urlStr string) string {
	matches := urlPasswordRegex.FindStringSubmatch(urlStr)
	if len(matches) > 1 {
		return matches[1]
	}
	return ""
}

// extractPasswordFromText 从文本提取密码
func (p *PiozPlugin) extractPasswordFromText(text string) string {
	matches := passwordRegex.FindStringSubmatch(text)
	if len(matches) > 1 {
		return matches[1]
	}
	return ""
}

// createLinkFromURL 从URL创建Link对象（使用对象池）
func (p *PiozPlugin) createLinkFromURL(urlStr, password string) model.Link {
	// ✅ 使用对象池减少内存分配
	link := p.linkPool.Get().(*model.Link)
	
	linkType := p.determineLinkType(urlStr)
	if linkType == "" {
		linkType = "other"
	}
	
	link.Type = linkType
	link.URL = urlStr
	link.Password = password
	
	// 复制值并归还对象到池
	result := *link
	
	// 清空对象并归还
	link.Type = ""
	link.URL = ""
	link.Password = ""
	p.linkPool.Put(link)
	
	return result
}

// containsLink 检查链接是否已存在
func (p *PiozPlugin) containsLink(links []model.Link, link model.Link) bool {
	for _, l := range links {
		if l.URL == link.URL {
			return true
		}
	}
	return false
}

// getCloudTypeName 获取云盘类型名称
func (p *PiozPlugin) getCloudTypeName(cloudType string) string {
	cloudType = strings.ToLower(cloudType)
	
	typeMap := map[string]string{
		"quark":      "夸克网盘",
		"baidu":      "百度网盘",
		"xunlei":     "迅雷云盘",
		"aliyun":     "阿里云盘",
		"uc":         "UC网盘",
		"lanzou":     "蓝奏云",
		"115":        "115网盘",
		"mobile":     "移动云盘",
		"weiyun":     "微云",
		"jianguoyun": "坚果云",
		"123":        "123云盘",
		"pikpak":     "PikPak",
		"tianyi":     "天翼云盘",
		"magnet":     "磁力链接",
		"ed2k":       "电驴链接",
	}
	
	if name, ok := typeMap[cloudType]; ok {
		return name
	}
	
	return cloudType
}

// GetPerformanceStats 获取性能统计信息
func (p *PiozPlugin) GetPerformanceStats() map[string]interface{} {
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
