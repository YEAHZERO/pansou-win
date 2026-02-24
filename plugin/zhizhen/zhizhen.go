package zhizhen

// ============================================================================
// Zhizhen 插件
// 数据源：xiaomi666.fun 搜索页 + 详情页
// 职责：抓取搜索结果并增强详情页链接与图片
// ============================================================================

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
	"sync/atomic"
	"time"

	"github.com/PuerkitoBio/goquery"
)

// 常量配置
const (
	DefaultTimeout = 8 * time.Second
	DetailTimeout  = 6 * time.Second

	MaxConcurrency = 20

	MaxIdleConns        = 200
	MaxIdleConnsPerHost = 50
	MaxConnsPerHost     = 100
	IdleConnTimeout     = 90 * time.Second

	cacheTTL = 1 * time.Hour
)

// 运行指标
var (
	searchRequests     int64 = 0
	detailPageRequests int64 = 0
	cacheHits          int64 = 0
	cacheMisses        int64 = 0
	totalSearchTime    int64 = 0
	totalDetailTime    int64 = 0
)

func init() {
	plugin.RegisterGlobalPlugin(NewZhizhenPlugin())
}

// 正则与详情缓存
var (
	detailIDRegex = regexp.MustCompile(`/vod/detail/id/(\d+)\.html`)

	passwordRegex = regexp.MustCompile(`\?pwd=([0-9a-zA-Z]+)`)

	quarkLinkRegex      = regexp.MustCompile(`https?://pan\.quark\.cn/s/[0-9a-zA-Z]+`)
	ucLinkRegex         = regexp.MustCompile(`https?://drive\.uc\.cn/s/[0-9a-zA-Z]+(\?[^"'\s]*)?`)
	baiduLinkRegex      = regexp.MustCompile(`https?://pan\.baidu\.com/s/[0-9a-zA-Z_\-]+(\?pwd=[0-9a-zA-Z]+)?`)
	aliyunLinkRegex     = regexp.MustCompile(`https?://(www\.)?(aliyundrive\.com|alipan\.com)/s/[0-9a-zA-Z]+`)
	xunleiLinkRegex     = regexp.MustCompile(`https?://pan\.xunlei\.com/s/[0-9a-zA-Z_\-]+(\?pwd=[0-9a-zA-Z]+)?`)
	tianyiLinkRegex     = regexp.MustCompile(`https?://cloud\.189\.cn/t/[0-9a-zA-Z]+`)
	link115Regex        = regexp.MustCompile(`https?://115\.com/s/[0-9a-zA-Z]+`)
	mobileLinkRegex     = regexp.MustCompile(`https?://caiyun\.feixin\.10086\.cn/[0-9a-zA-Z]+`)
	weiyunLinkRegex     = regexp.MustCompile(`https?://share\.weiyun\.com/[0-9a-zA-Z]+`)
	lanzouLinkRegex     = regexp.MustCompile(`https?://(www\.)?(lanzou[uixys]*|lan[zs]o[ux])\.(com|net|org)/[0-9a-zA-Z]+`)
	jianguoyunLinkRegex = regexp.MustCompile(`https?://(www\.)?jianguoyun\.com/p/[0-9a-zA-Z]+`)
	link123Regex        = regexp.MustCompile(`https?://123pan\.com/s/[0-9a-zA-Z]+`)
	pikpakLinkRegex     = regexp.MustCompile(`https?://mypikpak\.com/s/[0-9a-zA-Z]+`)
	magnetLinkRegex     = regexp.MustCompile(`magnet:\?xt=urn:btih:[0-9a-fA-F]{40}`)
	ed2kLinkRegex       = regexp.MustCompile(`ed2k://\|file\|.+\|\d+\|[0-9a-fA-F]{32}\|/`)

	detailCache = sync.Map{}
)

// 插件定义
type ZhizhenAsyncPlugin struct {
	*plugin.BaseAsyncPlugin
	optimizedClient *http.Client
}

// createOptimizedHTTPClient 创建带连接池的 HTTP 客户端。
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

// NewZhizhenPlugin 创建插件实例。
func NewZhizhenPlugin() *ZhizhenAsyncPlugin {
	return &ZhizhenAsyncPlugin{
		BaseAsyncPlugin: plugin.NewBaseAsyncPlugin("zhizhen", 2),
		optimizedClient: createOptimizedHTTPClient(),
	}
}

// Search 兼容基础搜索接口。
func (p *ZhizhenAsyncPlugin) Search(keyword string, ext map[string]interface{}) ([]model.SearchResult, error) {
	result, err := p.SearchWithResult(keyword, ext)
	if err != nil {
		return nil, err
	}
	return result.Results, nil
}

// SearchWithResult 走框架异步搜索入口。
func (p *ZhizhenAsyncPlugin) SearchWithResult(keyword string, ext map[string]interface{}) (model.PluginSearchResult, error) {
	return p.AsyncSearchWithResult(keyword, p.searchImpl, p.MainCacheKey, ext)
}

// searchImpl 主流程：抓取搜索页并执行详情增强。
func (p *ZhizhenAsyncPlugin) searchImpl(client *http.Client, keyword string, ext map[string]interface{}) ([]model.SearchResult, error) {

	start := time.Now()
	atomic.AddInt64(&searchRequests, 1)
	defer func() {
		duration := time.Since(start).Nanoseconds()
		atomic.AddInt64(&totalSearchTime, duration)
	}()

	if p.optimizedClient != nil {
		client = p.optimizedClient
	}

	searchURL := fmt.Sprintf("https://xiaomi666.fun/index.php/vod/search/wd/%s.html", url.QueryEscape(keyword))

	fmt.Printf("[%s] %s\n", p.Name(), "xiaomi666.fun")

	ctx, cancel := context.WithTimeout(context.Background(), DefaultTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", searchURL, nil)
	if err != nil {
		return nil, fmt.Errorf("[%s] 创建请求失败: %w", p.Name(), err)
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")
	req.Header.Set("Connection", "keep-alive")
	req.Header.Set("Upgrade-Insecure-Requests", "1")
	req.Header.Set("Cache-Control", "max-age=0")
	req.Header.Set("Referer", "https://xiaomi666.fun/")

	resp, err := p.doRequestWithRetry(req, client)
	if err != nil {
		return nil, fmt.Errorf("[%s] 搜索请求失败: %w", p.Name(), err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("[%s] 搜索请求返回状态码: %d", p.Name(), resp.StatusCode)
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("[%s] 解析搜索页面失败: %w", p.Name(), err)
	}

	var results []model.SearchResult

	doc.Find(".module-search-item").Each(func(i int, s *goquery.Selection) {
		result := p.parseSearchItem(s, keyword)
		if result.UniqueID != "" {
			results = append(results, result)
		}
	})

	enhancedResults := p.enhanceWithDetails(client, results)

	return plugin.FilterResultsByKeyword(enhancedResults, keyword), nil
}

// parseSearchItem 解析单条搜索结果。
func (p *ZhizhenAsyncPlugin) parseSearchItem(s *goquery.Selection, keyword string) model.SearchResult {
	result := model.SearchResult{}

	detailLink, exists := s.Find(".video-info-header h3 a").First().Attr("href")
	if !exists {
		return result
	}

	matches := detailIDRegex.FindStringSubmatch(detailLink)
	if len(matches) < 2 {
		return result
	}

	itemID := matches[1]
	result.UniqueID = fmt.Sprintf("%s-%s", p.Name(), itemID)

	titleElement := s.Find(".video-info-header h3 a")
	result.Title = strings.TrimSpace(titleElement.Text())

	qualityElement := s.Find(".video-serial")
	quality := strings.TrimSpace(qualityElement.Text())

	var tags []string
	s.Find(".video-info-aux .tag-link a").Each(func(i int, tag *goquery.Selection) {
		tagText := strings.TrimSpace(tag.Text())
		if tagText != "" {
			tags = append(tags, tagText)
		}
	})
	result.Tags = tags

	director := ""
	s.Find(".video-info-items").Each(func(i int, item *goquery.Selection) {
		title := strings.TrimSpace(item.Find(".video-info-itemtitle").Text())
		if strings.Contains(title, "导演") {
			director = strings.TrimSpace(item.Find(".video-info-actor a").Text())
		}
	})

	var actors []string
	s.Find(".video-info-items").Each(func(i int, item *goquery.Selection) {
		title := strings.TrimSpace(item.Find(".video-info-itemtitle").Text())
		if strings.Contains(title, "主演") {
			item.Find(".video-info-actor a").Each(func(j int, actor *goquery.Selection) {
				actorName := strings.TrimSpace(actor.Text())
				if actorName != "" {
					actors = append(actors, actorName)
				}
			})
		}
	})

	plotElement := s.Find(".video-info-items").FilterFunction(func(i int, item *goquery.Selection) bool {
		title := strings.TrimSpace(item.Find(".video-info-itemtitle").Text())
		return strings.Contains(title, "剧情")
	})
	plot := strings.TrimSpace(plotElement.Find(".video-info-item").Text())

	var images []string
	if picURL, exists := s.Find(".module-item-pic > img").Attr("data-src"); exists && picURL != "" {
		images = append(images, picURL)
	}
	result.Images = images

	var contentParts []string
	if quality != "" {
		contentParts = append(contentParts, "【"+quality+"】")
	}
	if director != "" {
		contentParts = append(contentParts, "导演："+director)
	}
	if len(actors) > 0 {
		actorStr := strings.Join(actors[:min(3, len(actors))], "、")
		if len(actors) > 3 {
			actorStr += "等"
		}
		contentParts = append(contentParts, "主演："+actorStr)
	}
	if plot != "" {
		contentParts = append(contentParts, plot)
	}

	result.Content = strings.Join(contentParts, "\n")
	result.Channel = ""
	result.Datetime = time.Time{}

	return result
}

// isValidNetworkDriveURL 判断是否为有效网盘链接。
func (p *ZhizhenAsyncPlugin) isValidNetworkDriveURL(url string) bool {

	if strings.Contains(url, "javascript:") ||
		strings.Contains(url, "#") ||
		url == "" ||
		(!strings.HasPrefix(url, "http") && !strings.HasPrefix(url, "magnet:") && !strings.HasPrefix(url, "ed2k:")) {
		return false
	}

	return quarkLinkRegex.MatchString(url) ||
		ucLinkRegex.MatchString(url) ||
		baiduLinkRegex.MatchString(url) ||
		aliyunLinkRegex.MatchString(url) ||
		xunleiLinkRegex.MatchString(url) ||
		tianyiLinkRegex.MatchString(url) ||
		link115Regex.MatchString(url) ||
		mobileLinkRegex.MatchString(url) ||
		weiyunLinkRegex.MatchString(url) ||
		lanzouLinkRegex.MatchString(url) ||
		jianguoyunLinkRegex.MatchString(url) ||
		link123Regex.MatchString(url) ||
		pikpakLinkRegex.MatchString(url) ||
		magnetLinkRegex.MatchString(url) ||
		ed2kLinkRegex.MatchString(url)
}

// determineLinkType 按正则判断链接类型。
func (p *ZhizhenAsyncPlugin) determineLinkType(url string) string {
	switch {
	case quarkLinkRegex.MatchString(url):
		return "quark"
	case ucLinkRegex.MatchString(url):
		return "uc"
	case baiduLinkRegex.MatchString(url):
		return "baidu"
	case aliyunLinkRegex.MatchString(url):
		return "aliyun"
	case xunleiLinkRegex.MatchString(url):
		return "xunlei"
	case tianyiLinkRegex.MatchString(url):
		return "tianyi"
	case link115Regex.MatchString(url):
		return "115"
	case mobileLinkRegex.MatchString(url):
		return "mobile"
	case weiyunLinkRegex.MatchString(url):
		return "weiyun"
	case lanzouLinkRegex.MatchString(url):
		return "lanzou"
	case jianguoyunLinkRegex.MatchString(url):
		return "jianguoyun"
	case link123Regex.MatchString(url):
		return "123"
	case pikpakLinkRegex.MatchString(url):
		return "pikpak"
	case magnetLinkRegex.MatchString(url):
		return "magnet"
	case ed2kLinkRegex.MatchString(url):
		return "ed2k"
	default:
		return ""
	}
}

// extractPassword 提取 URL 中的 pwd 参数。
func (p *ZhizhenAsyncPlugin) extractPassword(url string) string {
	matches := passwordRegex.FindStringSubmatch(url)
	if len(matches) > 1 {
		return matches[1]
	}
	return ""
}

// enhanceWithDetails 并发抓取详情页，补充链接和图片。
func (p *ZhizhenAsyncPlugin) enhanceWithDetails(client *http.Client, results []model.SearchResult) []model.SearchResult {
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

			parts := strings.Split(r.UniqueID, "-")
			if len(parts) < 2 {
				mu.Lock()
				enhancedResults = append(enhancedResults, r)
				mu.Unlock()
				return
			}

			itemID := parts[1]

			if cached, ok := detailCache.Load(itemID); ok {
				if cachedResult, ok := cached.(model.SearchResult); ok {
					atomic.AddInt64(&cacheHits, 1)
					mu.Lock()
					enhancedResults = append(enhancedResults, cachedResult)
					mu.Unlock()
					return
				}
			}
			atomic.AddInt64(&cacheMisses, 1)

			detailLinks, detailImages := p.fetchDetailLinksAndImages(client, itemID)
			r.Links = detailLinks

			if len(detailImages) > 0 {
				r.Images = detailImages
			}

			detailCache.Store(itemID, r)

			mu.Lock()
			enhancedResults = append(enhancedResults, r)
			mu.Unlock()
		}(result)
	}

	wg.Wait()
	return enhancedResults
}

// doRequestWithRetry 执行重试请求。
func (p *ZhizhenAsyncPlugin) doRequestWithRetry(req *http.Request, client *http.Client) (*http.Response, error) {
	maxRetries := 3
	var lastErr error

	for i := 0; i < maxRetries; i++ {
		if i > 0 {

			backoff := time.Duration(1<<uint(i-1)) * 200 * time.Millisecond
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

	return nil, fmt.Errorf("重试 %d 次后仍然失败: %w", maxRetries, lastErr)
}

// fetchDetailLinksAndImages 抓取详情页链接和封面图。
func (p *ZhizhenAsyncPlugin) fetchDetailLinksAndImages(client *http.Client, itemID string) ([]model.Link, []string) {

	start := time.Now()
	atomic.AddInt64(&detailPageRequests, 1)
	defer func() {
		duration := time.Since(start).Nanoseconds()
		atomic.AddInt64(&totalDetailTime, duration)
	}()

	detailURL := fmt.Sprintf("https://xiaomi666.fun/index.php/vod/detail/id/%s.html", itemID)

	ctx, cancel := context.WithTimeout(context.Background(), DetailTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", detailURL, nil)
	if err != nil {
		return nil, nil
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")
	req.Header.Set("Connection", "keep-alive")
	req.Header.Set("Referer", "https://xiaomi666.fun/")

	resp, err := p.doRequestWithRetry(req, client)
	if err != nil {
		return nil, nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, nil
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, nil
	}

	var links []model.Link
	var images []string

	if posterURL, exists := doc.Find(".mobile-play .lazyload").Attr("data-src"); exists && posterURL != "" {
		images = append(images, posterURL)
	}

	doc.Find("#download-list .module-row-one").Each(func(i int, s *goquery.Selection) {

		if linkURL, exists := s.Find("[data-clipboard-text]").Attr("data-clipboard-text"); exists {

			if p.isValidNetworkDriveURL(linkURL) {
				if linkType := p.determineLinkType(linkURL); linkType != "" {
					link := model.Link{
						Type:     linkType,
						URL:      linkURL,
						Password: "",
					}
					links = append(links, link)
				}
			}
		}

		s.Find("a[href]").Each(func(j int, a *goquery.Selection) {
			if linkURL, exists := a.Attr("href"); exists {

				if p.isValidNetworkDriveURL(linkURL) {
					if linkType := p.determineLinkType(linkURL); linkType != "" {

						isDuplicate := false
						for _, existingLink := range links {
							if existingLink.URL == linkURL {
								isDuplicate = true
								break
							}
						}

						if !isDuplicate {
							link := model.Link{
								Type:     linkType,
								URL:      linkURL,
								Password: "",
							}
							links = append(links, link)
						}
					}
				}
			}
		})
	})

	return links, images
}

// fetchDetailLinks 兼容旧接口，仅返回链接。
func (p *ZhizhenAsyncPlugin) fetchDetailLinks(client *http.Client, itemID string) []model.Link {
	links, _ := p.fetchDetailLinksAndImages(client, itemID)
	return links
}

// min 返回两个整数中的较小值。
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// GetPerformanceStats 返回运行时指标。
func (p *ZhizhenAsyncPlugin) GetPerformanceStats() map[string]interface{} {
	totalSearchRequests := atomic.LoadInt64(&searchRequests)
	totalDetailRequests := atomic.LoadInt64(&detailPageRequests)
	totalCacheHits := atomic.LoadInt64(&cacheHits)
	totalCacheMisses := atomic.LoadInt64(&cacheMisses)
	totalSearchTime := atomic.LoadInt64(&totalSearchTime)
	totalDetailTime := atomic.LoadInt64(&totalDetailTime)

	var avgSearchTime, avgDetailTime, cacheHitRate float64
	if totalSearchRequests > 0 {
		avgSearchTime = float64(totalSearchTime) / float64(totalSearchRequests) / 1e6
	}
	if totalDetailRequests > 0 {
		avgDetailTime = float64(totalDetailTime) / float64(totalDetailRequests) / 1e6
	}
	if totalCacheHits+totalCacheMisses > 0 {
		cacheHitRate = float64(totalCacheHits) / float64(totalCacheHits+totalCacheMisses) * 100
	}

	return map[string]interface{}{
		"search_requests":      totalSearchRequests,
		"detail_page_requests": totalDetailRequests,
		"cache_hits":           totalCacheHits,
		"cache_misses":         totalCacheMisses,
		"cache_hit_rate":       cacheHitRate,
		"avg_search_time_ms":   avgSearchTime,
		"avg_detail_time_ms":   avgDetailTime,
		"total_search_time_ns": totalSearchTime,
		"total_detail_time_ns": totalDetailTime,
	}
}
