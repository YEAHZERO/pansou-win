package pioz

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"pansou/model"
	"pansou/plugin"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"
)

const (
	// 默认超时时间
	DefaultTimeout = 10 * time.Second
	DetailTimeout  = 5 * time.Second

	// HTTP连接池配置
	MaxIdleConns        = 200
	MaxIdleConnsPerHost = 50
	MaxConnsPerHost     = 100
	IdleConnTimeout     = 90 * time.Second

	// 缓存TTL
	cacheTTL = 1 * time.Hour
)

func init() {
	plugin.RegisterGlobalPlugin(NewPiozPlugin())
}

// 预编译的正则表达式
var (
	// 夸克网盘链接的正则表达式
	quarkLinkRegex = regexp.MustCompile(`https?://pan\.quark\.cn/s/[0-9a-zA-Z]+`)
	
	// 密码提取正则表达式
	passwordRegex = regexp.MustCompile(`(?:提取码|密码)[：:]\s*([a-zA-Z0-9]{4})`)
	
	// 详情页链接正则
	detailLinkRegex = regexp.MustCompile(`/detail/(\d+)`)
	
	// 缓存相关
	searchResultCache = sync.Map{} // 缓存搜索结果
)

// PiozAsyncPlugin Pioz异步插件
type PiozAsyncPlugin struct {
	*plugin.BaseAsyncPlugin
	optimizedClient *http.Client
}

// createOptimizedHTTPClient 创建优化的HTTP客户端
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
	}
}

// NewPiozPlugin 创建新的Pioz异步插件
func NewPiozPlugin() *PiozAsyncPlugin {
	return &PiozAsyncPlugin{
		BaseAsyncPlugin: plugin.NewBaseAsyncPlugin("pioz", 1),
		optimizedClient: createOptimizedHTTPClient(),
	}
}

// Search 同步搜索接口
func (p *PiozAsyncPlugin) Search(keyword string, ext map[string]interface{}) ([]model.SearchResult, error) {
	result, err := p.SearchWithResult(keyword, ext)
	if err != nil {
		return nil, err
	}
	return result.Results, nil
}

// SearchWithResult 带结果统计的搜索接口
func (p *PiozAsyncPlugin) SearchWithResult(keyword string, ext map[string]interface{}) (model.PluginSearchResult, error) {
	return p.AsyncSearchWithResult(keyword, p.searchImpl, p.MainCacheKey, ext)
}

// searchImpl 实现具体的搜索逻辑
func (p *PiozAsyncPlugin) searchImpl(client *http.Client, keyword string, ext map[string]interface{}) ([]model.SearchResult, error) {
	// 检查缓存
	if cached, ok := searchResultCache.Load(keyword); ok {
		if cachedResp, ok := cached.(cachedResponse); ok {
			if time.Since(cachedResp.timestamp) < cacheTTL {
				return cachedResp.results, nil
			}
		}
	}

	// 使用优化的客户端
	if p.optimizedClient != nil {
		client = p.optimizedClient
	}

	// 1. 构建搜索URL
	searchURL := fmt.Sprintf("https://www.pioz.cn/search?q=%s", url.QueryEscape(keyword))
	
	// 记录搜索URL到日志
	fmt.Printf("[%s] %s\n", p.Name(), "www.pioz.cn")

	// 2. 创建带超时的上下文
	ctx, cancel := context.WithTimeout(context.Background(), DefaultTimeout)
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
	req.Header.Set("Referer", "https://www.pioz.cn/")

	// 5. 发送请求（带重试机制）
	resp, err := p.doRequestWithRetry(req, client)
	if err != nil {
		return nil, fmt.Errorf("[%s] 搜索请求失败: %w", p.Name(), err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("[%s] 搜索请求返回状态码: %d", p.Name(), resp.StatusCode)
	}

	// 6. 解析搜索结果页面
	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("[%s] 解析搜索页面失败: %w", p.Name(), err)
	}

	// 7. 提取搜索结果 - 根据实际HTML结构修改
	var results []model.SearchResult
	
	// 根据实际的HTML结构，搜索结果在 .file-item 或 .group 类中
	doc.Find(".file-item, .result-item, .search-item").Each(func(i int, s *goquery.Selection) {
		result := p.parseSearchItem(s, keyword, i)
		if result.UniqueID != "" {
			results = append(results, result)
		}
	})

	// 如果没找到，尝试更通用的选择器
	if len(results) == 0 {
		// 查找包含链接的div，这些通常是搜索结果
		doc.Find("div[class*='item'], a[href*='/detail/']").Each(func(i int, s *goquery.Selection) {
			// 如果直接找到详情链接，创建结果
			if s.Is("a") {
				if href, exists := s.Attr("href"); exists && strings.Contains(href, "/detail/") {
					result := p.parseFromLinkElement(s, keyword, i)
					if result.UniqueID != "" {
						results = append(results, result)
					}
				}
			} else {
				// 否则解析整个项目
				result := p.parseSearchItem(s, keyword, i)
				if result.UniqueID != "" {
					results = append(results, result)
				}
			}
		})
	}

	// 8. 异步获取详情页信息（二次跳转）
	enhancedResults := p.enhanceWithDetails(client, results)

	// 9. 缓存结果
	searchResultCache.Store(keyword, cachedResponse{
		results:   enhancedResults,
		timestamp: time.Now(),
	})

	// 10. 关键词过滤
	return plugin.FilterResultsByKeyword(enhancedResults, keyword), nil
}

// parseSearchItem 解析单个搜索结果项 - 根据实际HTML结构修改
func (p *PiozAsyncPlugin) parseSearchItem(s *goquery.Selection, keyword string, index int) model.SearchResult {
	result := model.SearchResult{}

	// 根据实际HTML结构提取标题
	// 实际页面中的标题在 <span class="text-gray-100"> 中
	title := ""
	
	// 尝试多种可能的标题选择器
	titleSelectors := []string{
		".text-gray-100",                    // 实际页面中的标题类
		"[class*='title']",                  // 包含title的类
		".hot-search-title-text",            // 热搜榜标题类
		".text-gray-100",                    // 搜索结果标题
		"span[title]",                       // 带title属性的span
	}
	
	for _, selector := range titleSelectors {
		if titleElem := s.Find(selector).First(); titleElem.Length() > 0 {
			title = strings.TrimSpace(titleElem.Text())
			// 如果没有文本内容，尝试title属性
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
		// 如果没有找到标题，尝试直接查找文本内容
		text := strings.TrimSpace(s.Text())
		if len(text) > 100 {
			title = text[:100] + "..."
		} else if text != "" {
			title = text
		}
	}

	if title == "" {
		return result // 没有标题，跳过
	}

	// 提取详情页链接
	var detailURL string
	s.Find("a[href]").Each(func(j int, a *goquery.Selection) {
		if href, exists := a.Attr("href"); exists {
			// 匹配详情页链接格式：/detail/数字
			if strings.Contains(href, "/detail/") {
				if strings.HasPrefix(href, "http") {
					detailURL = href
				} else {
					detailURL = "https://www.pioz.cn" + href
				}
				return // 找到就退出
			}
		}
	})

	// 如果没找到链接，但元素本身就是链接
	if detailURL == "" && s.Is("a") {
		if href, exists := s.Attr("href"); exists && strings.Contains(href, "/detail/") {
			if strings.HasPrefix(href, "http") {
				detailURL = href
			} else {
				detailURL = "https://www.pioz.cn" + href
			}
		}
	}

	// 提取描述/内容 - 根据实际页面结构调整
	content := ""
	
	// 实际页面中可能有文件大小、类型等信息
	s.Find(".text-gray-400, .text-sm, [class*='desc'], [class*='info']").Each(func(j int, elem *goquery.Selection) {
		text := strings.TrimSpace(elem.Text())
		if text != "" && !strings.Contains(text, "夸克网盘") && !strings.Contains(text, "2026-") {
			if content == "" {
				content = text
			} else {
				content += " | " + text
			}
		}
	})

	// 如果没有找到内容描述，使用一些基本信息
	if content == "" {
		// 提取来源和日期信息
		s.Find(".text-gray-400").Each(func(j int, elem *goquery.Selection) {
			text := strings.TrimSpace(elem.Text())
			if strings.Contains(text, "夸克网盘") || strings.Contains(text, "2026-") {
				if content == "" {
					content = text
				} else {
					content += " | " + text
				}
			}
		})
	}

	// 构建唯一ID
	if detailURL != "" {
		result.UniqueID = fmt.Sprintf("%s-detail-%s", p.Name(), url.QueryEscape(detailURL))
	} else {
		result.UniqueID = fmt.Sprintf("%s-html-%d-%d", p.Name(), index, time.Now().UnixNano())
	}

	result.Title = title
	result.Content = content
	result.Datetime = time.Time{} // 使用零值
	result.Links = []model.Link{}  // 初始化为空，稍后在enhanceWithDetails中填充
	result.Channel = ""            // 插件搜索结果必须为空字符串

	return result
}

// parseFromLinkElement 从链接元素直接解析结果
func (p *PiozAsyncPlugin) parseFromLinkElement(a *goquery.Selection, keyword string, index int) model.SearchResult {
	result := model.SearchResult{}

	// 提取标题
	title := strings.TrimSpace(a.Text())
	if title == "" {
		if titleAttr, exists := a.Attr("title"); exists {
			title = strings.TrimSpace(titleAttr)
		}
	}

	if title == "" {
		return result
	}

	// 提取详情页链接
	href, exists := a.Attr("href")
	if !exists || !strings.Contains(href, "/detail/") {
		return result
	}

	var detailURL string
	if strings.HasPrefix(href, "http") {
		detailURL = href
	} else {
		detailURL = "https://www.pioz.cn" + href
	}

	// 构建唯一ID
	result.UniqueID = fmt.Sprintf("%s-detail-%s", p.Name(), url.QueryEscape(detailURL))
	result.Title = title
	result.Content = ""
	result.Datetime = time.Time{}
	result.Links = []model.Link{}
	result.Channel = ""

	return result
}

// enhanceWithDetails 异步获取详情页信息以获取下载链接
func (p *PiozAsyncPlugin) enhanceWithDetails(client *http.Client, results []model.SearchResult) []model.SearchResult {
	var enhancedResults []model.SearchResult
	var mu sync.Mutex
	var wg sync.WaitGroup

	// 限制并发数
	const MaxConcurrency = 10 // 降低并发数避免被封
	semaphore := make(chan struct{}, MaxConcurrency)

	for _, result := range results {
		wg.Add(1)
		go func(r model.SearchResult) {
			defer wg.Done()

			// 获取信号量
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			// 从UniqueID中提取detailURL
			if strings.Contains(r.UniqueID, "-detail-") {
				parts := strings.SplitN(r.UniqueID, "-detail-", 2)
				if len(parts) == 2 {
					detailURL, err := url.QueryUnescape(parts[1])
					if err == nil && detailURL != "" {
						// 获取详情页链接
						links := p.fetchDetailPageLinks(client, detailURL)
						r.Links = links
						
						// 如果获取到了链接，记录日志
						if len(links) > 0 {
							fmt.Printf("[%s] 成功获取资源链接: %s -> %d个链接\n", 
								p.Name(), r.Title, len(links))
						}
					}
				}
			}

			mu.Lock()
			enhancedResults = append(enhancedResults, r)
			mu.Unlock()
		}(result)
	}

	wg.Wait()
	return enhancedResults
}

// fetchDetailPageLinks 访问详情页并提取真实的网盘链接
func (p *PiozAsyncPlugin) fetchDetailPageLinks(client *http.Client, detailURL string) []model.Link {
	var links []model.Link

	// 创建带超时的上下文
	ctx, cancel := context.WithTimeout(context.Background(), DetailTimeout)
	defer cancel()

	// 创建请求
	req, err := http.NewRequestWithContext(ctx, "GET", detailURL, nil)
	if err != nil {
		fmt.Printf("[%s] 创建详情页请求失败: %v\n", p.Name(), err)
		return links
	}

	// 设置请求头（模拟浏览器）
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")
	req.Header.Set("Referer", "https://www.pioz.cn/")
	req.Header.Set("Cookie", "same-site-cookie=1") // 添加Cookie避免反爬

	// 发送请求
	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("[%s] 访问详情页失败: %v\n", p.Name(), err)
		return links
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		fmt.Printf("[%s] 详情页返回状态码: %d\n", p.Name(), resp.StatusCode)
		return links
	}

	// 解析HTML
	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		fmt.Printf("[%s] 解析详情页失败: %v\n", p.Name(), err)
		return links
	}

	// 尝试多种方式提取链接
	
	// 1. 查找夸克网盘链接
	doc.Find("a[href*='pan.quark.cn']").Each(func(i int, s *goquery.Selection) {
		href, exists := s.Attr("href")
		if exists {
			link := model.Link{
				Type:     "quark",
				URL:      href,
				Password: "",
			}
			links = append(links, link)
		}
	})

	// 2. 如果没找到，查找包含"夸克网盘"文本的链接
	if len(links) == 0 {
		doc.Find("a").Each(func(i int, s *goquery.Selection) {
			text := strings.ToLower(s.Text())
			if strings.Contains(text, "夸克") || strings.Contains(text, "quark") {
				if href, exists := s.Attr("href"); exists {
					link := model.Link{
						Type:     "quark",
						URL:      href,
						Password: "",
					}
					links = append(links, link)
				}
			}
		})
	}

	// 3. 从页面文本中提取
	if len(links) == 0 {
		pageText := doc.Text()
		matches := quarkLinkRegex.FindAllString(pageText, -1)
		for _, match := range matches {
			link := model.Link{
				Type:     "quark",
				URL:      match,
				Password: "",
			}
			links = append(links, link)
		}
	}

	// 4. 查找阿里云盘等其他网盘
	if len(links) == 0 {
		doc.Find("a[href*='aliyundrive.com'], a[href*='123pan.com'], a[href*='baidu.com']").Each(func(i int, s *goquery.Selection) {
			href, exists := s.Attr("href")
			if exists {
				linkType := "other"
				if strings.Contains(href, "aliyundrive.com") {
					linkType = "aliyun"
				} else if strings.Contains(href, "123pan.com") {
					linkType = "123pan"
				} else if strings.Contains(href, "baidu.com") {
					linkType = "baidu"
				}
				
				link := model.Link{
					Type:     linkType,
					URL:      href,
					Password: "",
				}
				links = append(links, link)
			}
		})
	}

	// 提取密码
	if len(links) > 0 {
		pageText := doc.Text()
		if matches := passwordRegex.FindStringSubmatch(pageText); len(matches) > 1 {
			password := matches[1]
			for i := range links {
				links[i].Password = password
			}
		}
	}

	fmt.Printf("[%s] 从详情页提取到 %d 个链接\n", p.Name(), len(links))
	return links
}

// doRequestWithRetry 带重试机制的HTTP请求
func (p *PiozAsyncPlugin) doRequestWithRetry(req *http.Request, client *http.Client) (*http.Response, error) {
	maxRetries := 3
	var lastErr error

	for i := 0; i < maxRetries; i++ {
		if i > 0 {
			// 指数退避
			backoff := time.Duration(1<<uint(i-1)) * 500 * time.Millisecond
			time.Sleep(backoff)
			fmt.Printf("[%s] 第%d次重试搜索...\n", p.Name(), i+1)
		}

		// 克隆请求
		reqClone := req.Clone(req.Context())

		resp, err := client.Do(reqClone)
		if err == nil && resp.StatusCode == 200 {
			return resp, nil
		}

		if resp != nil {
			resp.Body.Close()
		}
		
		if err != nil {
			lastErr = err
		} else {
			lastErr = fmt.Errorf("HTTP状态码: %d", resp.StatusCode)
		}
	}

	return nil, fmt.Errorf("重试 %d 次后仍然失败: %w", maxRetries, lastErr)
}

// cachedResponse 缓存响应结构
type cachedResponse struct {
	results   []model.SearchResult
	timestamp time.Time
}