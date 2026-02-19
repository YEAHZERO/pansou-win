package xdpan

// ============================================================================
// Xdpan 插件
// 数据源：xiongdipan.com 搜索页 + 详情页
// 职责：抓取搜索结果并从详情页提取网盘链接
// ============================================================================

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"
	"pansou/model"
	"pansou/plugin"
)

// 常量配置
const (
	BaseURL        = "https://xiongdipan.com"
	UserAgent      = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36"
	MaxConcurrency = 10
	MaxRetries     = 3
)

// 调试配置
var (
	DebugLog = false
)

// 插件定义与详情缓存
type XdpanPlugin struct {
	*plugin.BaseAsyncPlugin
	detailCache sync.Map
	cacheTTL    time.Duration
}

// NewXdpanPlugin 创建插件实例。
func NewXdpanPlugin() *XdpanPlugin {
	return &XdpanPlugin{
		BaseAsyncPlugin: plugin.NewBaseAsyncPlugin("xdpan", 1),
		cacheTTL:        60 * time.Minute,
	}
}

// Search 兼容基础搜索接口。
func (p *XdpanPlugin) Search(keyword string, ext map[string]interface{}) ([]model.SearchResult, error) {
	result, err := p.SearchWithResult(keyword, ext)
	if err != nil {
		return nil, err
	}
	return result.Results, nil
}

// SearchWithResult 走框架异步搜索入口。
func (p *XdpanPlugin) SearchWithResult(keyword string, ext map[string]interface{}) (model.PluginSearchResult, error) {
	return p.AsyncSearchWithResult(keyword, p.searchImpl, p.MainCacheKey, ext)
}

// searchImpl 主流程：搜索、详情增强、关键词过滤。
func (p *XdpanPlugin) searchImpl(client *http.Client, keyword string, ext map[string]interface{}) ([]model.SearchResult, error) {
	if DebugLog {
		fmt.Printf("[xdpan] 开始搜索: keyword=%s\n", keyword)
	}

	searchResults, err := p.fetchSearchResults(client, keyword)
	if err != nil {
		if DebugLog {
			fmt.Printf("[xdpan] 获取搜索结果失败: %v\n", err)
		}
		return nil, fmt.Errorf("[%s] 获取搜索结果失败: %w", p.Name(), err)
	}
	if DebugLog {
		fmt.Printf("[xdpan] 获取搜索结果成功: 结果数=%d\n", len(searchResults))
	}

	p.enrichWithDetailInfo(client, searchResults)

	filteredResults := plugin.FilterResultsByKeyword(searchResults, keyword)
	if DebugLog {
		fmt.Printf("[xdpan] 关键词过滤后: 过滤前=%d, 过滤后=%d\n", len(searchResults), len(filteredResults))
	}

	return filteredResults, nil
}

// fetchSearchResults 请求搜索页并提取结果。
func (p *XdpanPlugin) fetchSearchResults(client *http.Client, keyword string) ([]model.SearchResult, error) {

	searchURL := fmt.Sprintf("%s/search?page=1&k=%s", BaseURL, url.QueryEscape(keyword))

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", searchURL, nil)
	if err != nil {
		return nil, fmt.Errorf("创建GET请求失败: %w", err)
	}

	p.setRequestHeaders(req)

	if DebugLog {
		fmt.Printf("[xdpan] %s\n", "www.xdpan.com")
	}

	resp, err := p.doRequestWithRetry(req, client)
	if err != nil {
		return nil, fmt.Errorf("GET请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("请求返回状态码: %d", resp.StatusCode)
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("解析HTML失败: %w", err)
	}

	return p.extractSearchResults(doc), nil
}

// extractSearchResults 从搜索页文档提取结果列表。
func (p *XdpanPlugin) extractSearchResults(doc *goquery.Document) []model.SearchResult {
	var results []model.SearchResult

	doc.Find("van-row").Each(func(i int, s *goquery.Selection) {

		detailLink := s.Find("a[href^='/s/']")
		if detailLink.Length() == 0 {
			return
		}

		result := p.parseSearchResult(s)
		if result.Title != "" {
			results = append(results, result)
			if DebugLog {
				fmt.Printf("[xdpan] 解析结果[%d]: title=%s, detailUrl=%s\n", i, result.Title, result.Content)
			}
		}
	})

	if DebugLog {
		fmt.Printf("[xdpan] 提取到有效结果数: %d\n", len(results))
	}

	return results
}

// parseSearchResult 解析单条搜索结果。
func (p *XdpanPlugin) parseSearchResult(s *goquery.Selection) model.SearchResult {

	detailLink := s.Find("a[href^='/s/']")
	detailPath, _ := detailLink.Attr("href")
	var detailURL string
	if detailPath != "" {
		detailURL = BaseURL + detailPath
	}

	resourceID := ""
	if detailPath != "" {
		parts := strings.Split(detailPath, "/")
		if len(parts) >= 3 {
			resourceID = parts[2]
		}
	}

	var titleParts []string
	s.Find("div[name='content-title'] span").Each(func(i int, span *goquery.Selection) {
		text := strings.TrimSpace(span.Text())
		if text != "" {
			titleParts = append(titleParts, text)
		}
	})
	title := strings.Join(titleParts, "")

	if title == "" {
		title = strings.TrimSpace(s.Find("div[name='content-title']").Text())
	}

	var shareTime, fileType string
	bottomText := s.Find("template").Text()
	if bottomText == "" {

		bottomText = s.Find("div").FilterFunction(func(i int, sel *goquery.Selection) bool {
			return strings.Contains(sel.Text(), "时间:")
		}).Text()
	}

	timeRegex := regexp.MustCompile(`时间:\s*(\d{4}-\d{1,2}-\d{1,2})`)
	if matches := timeRegex.FindStringSubmatch(bottomText); len(matches) > 1 {
		shareTime = matches[1]
	}

	formatRegex := regexp.MustCompile(`格式:\s*<b>([^<]+)</b>`)
	if matches := formatRegex.FindStringSubmatch(bottomText); len(matches) > 1 {
		fileType = matches[1]
	}

	parsedTime := p.parseTime(shareTime)

	content := fmt.Sprintf("类型: %s | 分享时间: %s | 详情: %s", fileType, shareTime, detailURL)

	if resourceID == "" {
		resourceID = fmt.Sprintf("%d", time.Now().UnixNano())
	}

	return model.SearchResult{
		MessageID: fmt.Sprintf("%s-%s", p.Name(), resourceID),
		UniqueID:  fmt.Sprintf("%s-%s", p.Name(), resourceID),
		Title:     title,
		Content:   content,
		Datetime:  parsedTime,
		Links:     []model.Link{},
		Channel:   "",
	}
}

// enrichWithDetailInfo 并发抓取详情页并补充链接。
func (p *XdpanPlugin) enrichWithDetailInfo(client *http.Client, results []model.SearchResult) {
	if len(results) == 0 {
		return
	}

	var wg sync.WaitGroup
	semaphore := make(chan struct{}, MaxConcurrency)

	for i := range results {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			time.Sleep(time.Duration(index%3) * 200 * time.Millisecond)

			detailURL := p.extractDetailURLFromContent(results[index].Content)
			if detailURL != "" {
				links := p.fetchDetailPageLinks(client, detailURL)
				if len(links) > 0 {
					results[index].Links = links
					if DebugLog {
						fmt.Printf("[xdpan] 获取详情页链接成功: %s, 链接数: %d\n", detailURL, len(links))
					}
				}
			}
		}(i)
	}

	wg.Wait()
}

// fetchDetailPageLinks 获取详情页链接并使用缓存。
func (p *XdpanPlugin) fetchDetailPageLinks(client *http.Client, detailURL string) []model.Link {
	if detailURL == "" {
		return []model.Link{}
	}

	if cached, ok := p.detailCache.Load(detailURL); ok {
		if cacheItem, ok := cached.(cacheItem); ok {
			if time.Since(cacheItem.timestamp) < p.cacheTTL {
				return cacheItem.links
			}
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", detailURL, nil)
	if err != nil {
		return []model.Link{}
	}

	p.setRequestHeaders(req)

	resp, err := client.Do(req)
	if err != nil {
		return []model.Link{}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return []model.Link{}
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return []model.Link{}
	}

	links := p.extractDetailPageLinks(doc)

	p.detailCache.Store(detailURL, cacheItem{
		links:     links,
		timestamp: time.Now(),
	})

	return links
}

// extractDetailPageLinks 从详情页脚本中提取网盘链接。
func (p *XdpanPlugin) extractDetailPageLinks(doc *goquery.Document) []model.Link {
	var links []model.Link

	password := ""
	doc.Find("van-cell").Each(func(i int, s *goquery.Selection) {
		title, _ := s.Attr("title")
		if title == "密码" {
			password = strings.TrimSpace(s.Find("b").Text())
		}
	})

	doc.Find("script").Each(func(i int, s *goquery.Selection) {
		scriptContent := s.Text()

		re := regexp.MustCompile(`window\.open\("([^"]*pan\.baidu\.com[^"]*)"`)
		matches := re.FindStringSubmatch(scriptContent)

		if len(matches) > 1 {
			baiduURL := matches[1]

			if !strings.Contains(baiduURL, "pwd=") && password != "" {
				separator := "?"
				if strings.Contains(baiduURL, "?") {
					separator = "&"
				}
				baiduURL = fmt.Sprintf("%s%spwd=%s", baiduURL, separator, password)
			}

			links = append(links, model.Link{
				URL:      baiduURL,
				Type:     "baidu",
				Password: password,
			})

			if DebugLog {
				fmt.Printf("[xdpan] 提取到百度网盘链接: %s, 密码: %s\n", baiduURL, password)
			}
		}
	})

	return links
}

// extractDetailURLFromContent 从内容字段回提详情 URL。
func (p *XdpanPlugin) extractDetailURLFromContent(content string) string {

	re := regexp.MustCompile(`详情:\s*(https?://[^\s]+)`)
	matches := re.FindStringSubmatch(content)
	if len(matches) > 1 {
		return matches[1]
	}
	return ""
}

// parseTime 解析时间字符串，失败时回退当前时间。
func (p *XdpanPlugin) parseTime(timeStr string) time.Time {
	timeStr = strings.TrimSpace(timeStr)
	if timeStr == "" {
		return time.Now()
	}

	formats := []string{
		"2006-1-2",
		"2006-01-02",
		"2006-1-2 15:04",
		"2006-01-02 15:04",
		"2006-1-2 15:04:05",
		"2006-01-02 15:04:05",
	}

	for _, format := range formats {
		if t, err := time.Parse(format, timeStr); err == nil {
			return t
		}
	}

	return time.Now()
}

// doRequestWithRetry 带指数退避的重试请求。
func (p *XdpanPlugin) doRequestWithRetry(req *http.Request, client *http.Client) (*http.Response, error) {
	var lastErr error

	for i := 0; i < MaxRetries; i++ {
		if i > 0 {

			backoff := time.Duration(1<<uint(i-1)) * 500 * time.Millisecond
			if DebugLog {
				fmt.Printf("[xdpan] 重试第%d次，等待%v\n", i, backoff)
			}
			time.Sleep(backoff)
		}

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

	return nil, fmt.Errorf("重试 %d 次后仍然失败: %w", MaxRetries, lastErr)
}

// setRequestHeaders 设置请求头。
func (p *XdpanPlugin) setRequestHeaders(req *http.Request) {
	req.Header.Set("User-Agent", UserAgent)
	req.Header.Set("Referer", BaseURL+"/")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")
	req.Header.Set("Connection", "keep-alive")
	req.Header.Set("Cache-Control", "max-age=0")
}

// 详情缓存项
type cacheItem struct {
	links     []model.Link
	timestamp time.Time
}

// init 注册插件。
func init() {
	p := NewXdpanPlugin()
	plugin.RegisterGlobalPlugin(p)
}
