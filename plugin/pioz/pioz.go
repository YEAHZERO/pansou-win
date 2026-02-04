package pioz

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"
	"pansou/model"
	"pansou/plugin"
	"pansou/util/json"
)

// 预编译正则表达式
var (
	// 夸克网盘链接的正则表达式
	quarkLinkRegex = regexp.MustCompile(`https?://pan\.quark\.cn/s/[0-9a-zA-Z]+`)
	
	// 密码提取正则表达式
	passwordRegex = regexp.MustCompile(`(?:提取码|密码)[：:]\s*([a-zA-Z0-9]{4})`)
	
	// 缓存相关变量
	searchResultCache = sync.Map{}
	lastCacheCleanTime = time.Now()
	cacheTTL = 1 * time.Hour
)

// 在init函数中注册插件
func init() {
	plugin.RegisterGlobalPlugin(NewPiozPlugin())
	
	// 启动缓存清理goroutine
	go startCacheCleaner()
}

// startCacheCleaner 启动一个定期清理缓存的goroutine
func startCacheCleaner() {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()
	
	for range ticker.C {
		searchResultCache = sync.Map{}
		lastCacheCleanTime = time.Now()
	}
}

// 缓存响应结构
type cachedResponse struct {
	results   []model.SearchResult
	timestamp time.Time
}

const (
	// 网站基础URL - 夸克小站
	WebsiteURL = "https://www.pioz.cn"
	SearchURL  = "https://www.pioz.cn/search"

	// 默认参数
	DefaultTimeout = 10 * time.Second
	PageSize       = 20
	MaxResults     = 200
	MaxRetries     = 3
)

// PiozAsyncPlugin Pioz异步插件（夸克小站）
type PiozAsyncPlugin struct {
	*plugin.BaseAsyncPlugin
	timeout         time.Duration
	maxResults      int
	retries         int
	optimizedClient *http.Client
}

// createOptimizedHTTPClient 创建优化的HTTP客户端
func createOptimizedHTTPClient() *http.Client {
	transport := &http.Transport{
		MaxIdleConns:        200,
		MaxIdleConnsPerHost: 50,
		MaxConnsPerHost:     100,
		IdleConnTimeout:     90 * time.Second,
		DisableKeepAlives:   false,
	}
	return &http.Client{Transport: transport, Timeout: DefaultTimeout}
}

// NewPiozPlugin 创建新的Pioz异步插件
func NewPiozPlugin() *PiozAsyncPlugin {
	return &PiozAsyncPlugin{
		BaseAsyncPlugin: plugin.NewBaseAsyncPlugin("pioz", 1),
		timeout:         DefaultTimeout,
		maxResults:      MaxResults,
		retries:         MaxRetries,
		optimizedClient: createOptimizedHTTPClient(),
	}
}

// Search 执行搜索并返回结果（兼容性方法）
func (p *PiozAsyncPlugin) Search(keyword string, ext map[string]interface{}) ([]model.SearchResult, error) {
	result, err := p.SearchWithResult(keyword, ext)
	if err != nil {
		return nil, err
	}
	return result.Results, nil
}

// SearchWithResult 执行搜索并返回包含IsFinal标记的结果
func (p *PiozAsyncPlugin) SearchWithResult(keyword string, ext map[string]interface{}) (model.PluginSearchResult, error) {
	return p.AsyncSearchWithResult(keyword, p.doSearch, p.MainCacheKey, ext)
}

// doSearch 执行具体的搜索逻辑
func (p *PiozAsyncPlugin) doSearch(client *http.Client, keyword string, ext map[string]interface{}) ([]model.SearchResult, error) {
	// 检查缓存
	if cached, ok := searchResultCache.Load(keyword); ok {
		cachedResp := cached.(cachedResponse)
		if time.Since(cachedResp.timestamp) < cacheTTL {
			return cachedResp.results, nil
		}
	}

	// 使用优化的客户端
	if p.optimizedClient != nil {
		client = p.optimizedClient
	}

	// 1. 构建搜索URL - 使用GET参数
	searchURL := fmt.Sprintf("%s?q=%s", SearchURL, url.QueryEscape(keyword))
	
	// 记录搜索URL到日志
	fmt.Printf("[%s] %s\n", p.Name(), "www.pioz.cn")

	// 2. 创建带超时的上下文
	ctx, cancel := context.WithTimeout(context.Background(), p.timeout)
	defer cancel()

	// 3. 创建请求
	req, err := http.NewRequestWithContext(ctx, "GET", searchURL, nil)
	if err != nil {
		return nil, fmt.Errorf("[%s] 创建请求失败: %w", p.Name(), err)
	}

	// 4. 设置完整的请求头（避免反爬虫）
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")
	req.Header.Set("Connection", "keep-alive")
	req.Header.Set("Upgrade-Insecure-Requests", "1")
	req.Header.Set("Cache-Control", "max-age=0")
	req.Header.Set("Referer", WebsiteURL)

	// 5. 发送请求（带重试机制）
	resp, err := p.doRequestWithRetry(req, client)
	if err != nil {
		return nil, fmt.Errorf("[%s] 搜索请求失败: %w", p.Name(), err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("[%s] 服务器返回非200状态码: %d", p.Name(), resp.StatusCode)
	}

	// 6. 读取响应体
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("[%s] 读取响应失败: %w", p.Name(), err)
	}

	// 7. 尝试解析为JSON（如果是API响应）
	var results []model.SearchResult
	
	// 先尝试JSON解析
	var apiResp PiozAPIResponse
	if err := json.Unmarshal(respBody, &apiResp); err == nil && apiResp.Code == 200 {
		// JSON API响应
		results = p.convertAPIResults(apiResp.Data, keyword)
	} else {
		// HTML响应，使用goquery解析
		results, err = p.parseHTMLResults(respBody, keyword)
		if err != nil {
			return nil, fmt.Errorf("[%s] 解析HTML失败: %w", p.Name(), err)
		}
	}

	// 8. 缓存结果
	searchResultCache.Store(keyword, cachedResponse{
		results:   results,
		timestamp: time.Now(),
	})

	// 9. 关键词过滤
	return plugin.FilterResultsByKeyword(results, keyword), nil
}

// parseHTMLResults 解析HTML响应
func (p *PiozAsyncPlugin) parseHTMLResults(htmlBody []byte, keyword string) ([]model.SearchResult, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(string(htmlBody)))
	if err != nil {
		return nil, err
	}

	var results []model.SearchResult
	
	// 尝试多种可能的选择器
	// 选择器1: 通用的搜索结果项
	doc.Find(".search-result-item, .result-item, .item, .list-item").Each(func(i int, s *goquery.Selection) {
		result := p.parseSearchItem(s, keyword, i)
		if result.UniqueID != "" && len(result.Links) > 0 {
			results = append(results, result)
		}
	})

	// 如果没有找到结果，尝试其他选择器
	if len(results) == 0 {
		doc.Find("div[class*='result'], div[class*='item'], li[class*='result'], li[class*='item']").Each(func(i int, s *goquery.Selection) {
			result := p.parseSearchItem(s, keyword, i)
			if result.UniqueID != "" && len(result.Links) > 0 {
				results = append(results, result)
			}
		})
	}

	return results, nil
}

// parseSearchItem 解析单个搜索结果项
func (p *PiozAsyncPlugin) parseSearchItem(s *goquery.Selection, keyword string, index int) model.SearchResult {
	result := model.SearchResult{}

	// 提取标题
	title := ""
	titleSelectors := []string{".title", "h3", "h4", ".name", "[class*='title']"}
	for _, selector := range titleSelectors {
		if titleElem := s.Find(selector).First(); titleElem.Length() > 0 {
			title = strings.TrimSpace(titleElem.Text())
			if title != "" {
				break
			}
		}
	}

	if title == "" {
		return result // 没有标题，跳过
	}

	// 提取链接
	var links []model.Link
	s.Find("a[href]").Each(func(j int, a *goquery.Selection) {
		if href, exists := a.Attr("href"); exists {
			if quarkLinkRegex.MatchString(href) {
				link := model.Link{
					Type:     "quark",
					URL:      href,
					Password: "",
				}
				links = append(links, link)
			}
		}
	})

	// 如果没有找到链接，尝试从文本中提取
	if len(links) == 0 {
		text := s.Text()
		matches := quarkLinkRegex.FindAllString(text, -1)
		for _, match := range matches {
			link := model.Link{
				Type:     "quark",
				URL:      match,
				Password: "",
			}
			links = append(links, link)
		}
	}

	if len(links) == 0 {
		return result // 没有链接，跳过
	}

	// 提取描述
	content := ""
	contentSelectors := []string{".description", ".desc", ".content", "p", "[class*='desc']"}
	for _, selector := range contentSelectors {
		if contentElem := s.Find(selector).First(); contentElem.Length() > 0 {
			content = strings.TrimSpace(contentElem.Text())
			if content != "" {
				break
			}
		}
	}

	// 尝试提取密码
	fullText := s.Text()
	if matches := passwordRegex.FindStringSubmatch(fullText); len(matches) > 1 {
		for i := range links {
			links[i].Password = matches[1]
		}
	}

	// 构建唯一ID
	uniqueID := fmt.Sprintf("%s-html-%d", p.Name(), index)

	result = model.SearchResult{
		UniqueID: uniqueID,
		Title:    title,
		Content:  content,
		Datetime: time.Time{}, // 使用零值
		Links:    links,
		Channel:  "", // 插件搜索结果必须为空字符串
	}

	return result
}

// convertAPIResults 将API响应转换为标准SearchResult格式
func (p *PiozAsyncPlugin) convertAPIResults(items []PiozItem, keyword string) []model.SearchResult {
	results := make([]model.SearchResult, 0, len(items))

	for _, item := range items {
		// 提取链接信息
		linkInfo := p.extractLinkInfo(item)
		if linkInfo.URL == "" {
			continue // 跳过没有有效链接的项
		}

		// 创建链接
		link := model.Link{
			URL:      linkInfo.URL,
			Type:     linkInfo.Type,
			Password: linkInfo.Password,
		}

		// 创建唯一ID
		uniqueID := fmt.Sprintf("%s-%s", p.Name(), item.ID)

		// 解析时间
		var datetime time.Time
		if item.CreateTime != "" {
			// 尝试多种时间格式
			timeFormats := []string{
				"2006-01-02 15:04:05",
				"2006-01-02T15:04:05Z",
				time.RFC3339,
			}
			for _, format := range timeFormats {
				if parsedTime, err := time.Parse(format, item.CreateTime); err == nil {
					datetime = parsedTime
					break
				}
			}
		}

		// 创建搜索结果
		result := model.SearchResult{
			UniqueID: uniqueID,
			Title:    item.Title,
			Content:  item.Description,
			Datetime: datetime,
			Links:    []model.Link{link},
			Channel:  "", // 插件搜索结果必须为空字符串
		}

		results = append(results, result)
	}

	return results
}

// LinkInfo 链接信息
type LinkInfo struct {
	URL      string
	Type     string
	Password string
}

// extractLinkInfo 从项目中提取链接信息
func (p *PiozAsyncPlugin) extractLinkInfo(item PiozItem) LinkInfo {
	linkInfo := LinkInfo{}

	// 从URL字段提取链接
	if item.URL != "" {
		linkInfo.URL = item.URL
		linkInfo.Type = p.detectLinkType(item.URL)
	}

	// 从内容中提取密码
	if item.Password != "" {
		linkInfo.Password = item.Password
	} else {
		// 尝试从描述中提取密码
		matches := passwordRegex.FindStringSubmatch(item.Description)
		if len(matches) > 1 {
			linkInfo.Password = matches[1]
		}
	}

	return linkInfo
}

// detectLinkType 检测链接类型
func (p *PiozAsyncPlugin) detectLinkType(url string) string {
	url = strings.ToLower(url)

	if strings.Contains(url, "pan.quark.cn") || strings.Contains(url, "quark.cn") {
		return "quark"
	} else if strings.Contains(url, "drive.uc.cn") {
		return "uc"
	} else if strings.Contains(url, "pan.baidu.com") {
		return "baidu"
	} else if strings.Contains(url, "aliyundrive.com") || strings.Contains(url, "alipan.com") {
		return "aliyun"
	} else if strings.Contains(url, "cloud.189.cn") {
		return "tianyi"
	} else if strings.Contains(url, "pan.xunlei.com") {
		return "xunlei"
	} else if strings.Contains(url, "115.com") {
		return "115"
	} else if strings.Contains(url, "mypikpak.com") {
		return "pikpak"
	} else if strings.Contains(url, "caiyun.139.com") {
		return "mobile"
	} else if strings.Contains(url, "123pan.com") || strings.Contains(url, "123yunpan.com") {
		return "123"
	}

	return "others"
}

// doRequestWithRetry 带重试机制的HTTP请求
func (p *PiozAsyncPlugin) doRequestWithRetry(req *http.Request, client *http.Client) (*http.Response, error) {
	var lastErr error

	for i := 0; i < p.retries; i++ {
		if i > 0 {
			// 指数退避重试
			backoff := time.Duration(1<<uint(i-1)) * 200 * time.Millisecond
			time.Sleep(backoff)
		}

		// 克隆请求避免并发问题
		reqClone := req.Clone(req.Context())

		resp, err := client.Do(reqClone)
		if err == nil && resp.StatusCode == 200 {
			return resp, nil
		}

		if resp != nil {
			resp.Body.Close()
		}
		lastErr = err
	}

	return nil, fmt.Errorf("重试 %d 次后仍然失败: %w", p.retries, lastErr)
}

// API响应结构（JSON格式）
type PiozAPIResponse struct {
	Code    int        `json:"code"`
	Message string     `json:"message"`
	Data    []PiozItem `json:"data"`
	Total   int        `json:"total"`
}

// PiozItem API响应中的单个结果项
type PiozItem struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	URL         string `json:"url"`
	Password    string `json:"password"`
	CreateTime  string `json:"create_time"`
	PanType     string `json:"pan_type"`
}
