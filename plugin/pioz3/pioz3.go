package pioz3

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
)

const (
	DefaultTimeout = 15 * time.Second
	DetailTimeout  = 12 * time.Second

	SiteBaseURL = "https://www.pioz.cn"
	APIBaseURL  = "https://www.pioz.cn/api"

	RequestDelayMin = 1000 * time.Millisecond
	RequestDelayMax = 2000 * time.Millisecond

	DeepSearchActionID = "406ba109bf31d3d81924420f284be8a50f5694e5fc"
	DeepLinkActionID   = "406d8d103081c9476f011dffd6e75b43e160f7bc29"

	MaxConcurrency       = 8
	MaxEnhancedResults   = 50
	CacheTTL             = 30 * time.Minute
	KeywordIntervalMin   = 15000 * time.Millisecond
	KeywordIntervalMax   = 20000 * time.Millisecond
	UARotationInterval   = 3
)

var (
	quarkLinkRegex      = regexp.MustCompile(`https?://pan\.quark\.cn/s/[0-9a-zA-Z]{6,}`)
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

	searchCache   sync.Map
	detailCache   sync.Map
	lastKeyword   string
	lastKeywordTime time.Time
	requestCounter int64
	lastRequestTime time.Time

	searchRequests  int64
	cacheHits       int64
	cacheMisses     int64
	totalSearchTime int64
)

type cachedResponse struct {
	results   []model.SearchResult
	timestamp time.Time
}

type DeepSearchResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Total   int    `json:"total"`
	Results []struct {
		ID        string `json:"id"`
		Title     string `json:"title"`
		URL       string `json:"url"`
		CloudType string `json:"cloud_type"`
		Password  string `json:"password"`
		Datetime  string `json:"datetime"`
	} `json:"results"`
}

type Pioz3AsyncPlugin struct {
	*plugin.BaseAsyncPlugin
	optimizedClient *http.Client
	userAgents      []string

	stateMu          sync.Mutex
	currentUserAgent string

	sessionMu      sync.RWMutex
	sessionCookies []*http.Cookie
}

func createOptimizedHTTPClient() *http.Client {
	transport := &http.Transport{
		MaxIdleConns:        50,
		MaxIdleConnsPerHost: 10,
		MaxConnsPerHost:     20,
		IdleConnTimeout:     60 * time.Second,
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

func NewPioz3Plugin() *Pioz3AsyncPlugin {
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
	return &Pioz3AsyncPlugin{
		BaseAsyncPlugin:  plugin.NewBaseAsyncPluginWithFilter("pioz3", 1, true),
		optimizedClient:  createOptimizedHTTPClient(),
		userAgents:       userAgents,
		currentUserAgent: userAgents[randomIndex],
	}
}

func init() {
	plugin.RegisterGlobalPlugin(NewPioz3Plugin())
}

func (p *Pioz3AsyncPlugin) Search(keyword string, ext map[string]interface{}) ([]model.SearchResult, error) {
	result, err := p.SearchWithResult(keyword, ext)
	if err != nil {
		return nil, err
	}
	return result.Results, nil
}

func (p *Pioz3AsyncPlugin) SearchWithResult(keyword string, ext map[string]interface{}) (model.PluginSearchResult, error) {
	return p.AsyncSearchWithResult(keyword, p.searchImpl, p.MainCacheKey, ext)
}

func (p *Pioz3AsyncPlugin) searchImpl(client *http.Client, keyword string, ext map[string]interface{}) ([]model.SearchResult, error) {
	start := time.Now()
	atomic.AddInt64(&searchRequests, 1)
	defer func() {
		duration := time.Since(start).Nanoseconds()
		atomic.AddInt64(&totalSearchTime, duration)
	}()

	if lastKeyword != keyword && lastKeyword != "" {
		timeSinceLastKeyword := time.Since(lastKeywordTime)
		keywordInterval := KeywordIntervalMin + time.Duration(time.Now().UnixNano()%int64(KeywordIntervalMax-KeywordIntervalMin))
		if timeSinceLastKeyword < keywordInterval {
			delay := keywordInterval - timeSinceLastKeyword
			fmt.Printf("[%s] [反爬虫] 不同关键词搜索间隔: 等待 %.1f 秒\n", p.Name(), delay.Seconds())
			fmt.Printf("[%s] [反爬虫] 上次关键词: %s, 当前关键词: %s\n", p.Name(), lastKeyword, keyword)
			time.Sleep(delay)
		}
	}
	lastKeyword = keyword
	lastKeywordTime = time.Now()
	fmt.Printf("[%s] [搜索] 开始搜索关键词: %s\n", p.Name(), keyword)

	cacheKey := fmt.Sprintf("%s:%s", p.Name(), keyword)
	if cached, ok := searchCache.Load(cacheKey); ok {
		if cachedResp, ok := cached.(cachedResponse); ok {
			if time.Since(cachedResp.timestamp) < CacheTTL {
				atomic.AddInt64(&cacheHits, 1)
				fmt.Printf("[%s] [缓存] 命中搜索缓存\n", p.Name())
				return cachedResp.results, nil
			}
		}
	}
	atomic.AddInt64(&cacheMisses, 1)

	if p.optimizedClient != nil {
		client = p.optimizedClient
	}

	p.applyAntiCrawlerDelay()

	results, err := p.performDeepSearchViaServerAction(client, keyword)
	if err == nil && len(results) > 0 {
		enhancedResults := p.enhanceDeepSearchResults(client, results)
		searchCache.Store(cacheKey, cachedResponse{
			results:   enhancedResults,
			timestamp: time.Now(),
		})
		return enhancedResults, nil
	}
	fmt.Printf("[%s] [搜索] 深度搜索ServerAction失败: %v\n", p.Name(), err)

	p.applyAntiCrawlerDelay()
	results, err = p.performDeepSearch(client, keyword)
	if err == nil && len(results) > 0 {
		enhancedResults := p.enhanceDeepSearchResults(client, results)
		searchCache.Store(cacheKey, cachedResponse{
			results:   enhancedResults,
			timestamp: time.Now(),
		})
		return enhancedResults, nil
	}

	if err != nil {
		fmt.Printf("[%s] [搜索] API深度搜索失败: %v\n", p.Name(), err)
	}
	return []model.SearchResult{}, nil
}

func (p *Pioz3AsyncPlugin) applyAntiCrawlerDelay() {
	p.stateMu.Lock()
	defer p.stateMu.Unlock()

	now := time.Now()
	timeSinceLast := now.Sub(lastRequestTime)

	minDelay := RequestDelayMin + time.Duration(time.Now().UnixNano()%int64(RequestDelayMax-RequestDelayMin))
	if timeSinceLast < minDelay {
		delay := minDelay - timeSinceLast
		randomDelay := RequestDelayMin + time.Duration(time.Now().UnixNano()%int64(RequestDelayMax-RequestDelayMin))
		totalDelay := delay + randomDelay

		if totalDelay > 0 {
			fmt.Printf("[%s] [反爬虫] 请求延迟: %.2f秒 (距离上次请求 %.2f秒)\n", p.Name(), totalDelay.Seconds(), timeSinceLast.Seconds())
			time.Sleep(totalDelay)
		}
	}

	lastRequestTime = time.Now()

	requestCount := atomic.AddInt64(&requestCounter, 1)
	fmt.Printf("[%s] [反爬虫] 当前请求计数: %d\n", p.Name(), requestCount)

	if requestCount%UARotationInterval == 0 {
		randomIndex := time.Now().UnixNano() % int64(len(p.userAgents))
		p.currentUserAgent = p.userAgents[randomIndex]
		fmt.Printf("[%s] [反爬虫] 第%d次请求，轮换User-Agent: %s\n", p.Name(), requestCount, p.currentUserAgent)
	}
}

func (p *Pioz3AsyncPlugin) currentUA() string {
	p.stateMu.Lock()
	defer p.stateMu.Unlock()
	return p.currentUserAgent
}

func (p *Pioz3AsyncPlugin) performDeepSearchViaServerAction(client *http.Client, keyword string) ([]model.SearchResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), DefaultTimeout)
	defer cancel()

	body := fmt.Sprintf(`["%s"]`, keyword)
	req, err := http.NewRequestWithContext(ctx, "POST", SiteBaseURL+"/", strings.NewReader(body))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Accept", "text/x-component")
	req.Header.Set("Content-Type", "text/plain;charset=UTF-8")
	req.Header.Set("next-action", DeepSearchActionID)
	p.setStealthHeaders(req)
	p.addSessionCookies(req)

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	p.sessionMu.Lock()
	p.sessionCookies = resp.Cookies()
	p.sessionMu.Unlock()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("server action status: %d", resp.StatusCode)
	}

	respBody, err := p.readCompressedBody(resp)
	if err != nil {
		return nil, err
	}

	results := p.parseDeepSearchServerActionResponse(respBody)
	if len(results) == 0 {
		return nil, fmt.Errorf("server action has no results")
	}
	fmt.Printf("[%s] [搜索] 深度搜索(ServerAction)找到 %d 个结果\n", p.Name(), len(results))
	return results, nil
}

func (p *Pioz3AsyncPlugin) performDeepSearch(client *http.Client, keyword string) ([]model.SearchResult, error) {
	apiURL := fmt.Sprintf("%s/deep-search?kw=%s", APIBaseURL, url.QueryEscape(keyword))

	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return nil, err
	}

	p.setAPIHeaders(req)
	p.addSessionCookies(req)

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("API返回状态码: %d", resp.StatusCode)
	}

	body, err := p.readCompressedBody(resp)
	if err != nil {
		return nil, err
	}

	var apiResp DeepSearchResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return nil, fmt.Errorf("解析API响应失败: %w", err)
	}

	if apiResp.Code != 0 {
		return nil, fmt.Errorf("API错误: %s", apiResp.Message)
	}

	if len(apiResp.Results) == 0 {
		return nil, fmt.Errorf("未找到搜索结果")
	}

	var results []model.SearchResult
	for _, item := range apiResp.Results {
		result := p.convertAPIResultToSearchResult(item)
		results = append(results, result)
	}

	fmt.Printf("[%s] [搜索] 深度搜索(API)找到 %d 个结果\n", p.Name(), len(results))
	return results, nil
}

func (p *Pioz3AsyncPlugin) convertAPIResultToSearchResult(item struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	URL       string `json:"url"`
	CloudType string `json:"cloud_type"`
	Password  string `json:"password"`
	Datetime  string `json:"datetime"`
}) model.SearchResult {
	content := fmt.Sprintf("来源: %s | 深度搜索API", p.getCloudTypeName(item.CloudType))

	var dt time.Time
	if item.Datetime != "" {
		if parsedTime, err := time.Parse("2006-01-02 15:04", item.Datetime); err == nil {
			dt = parsedTime
		} else if parsedTime, err := time.Parse("2006-01-02", item.Datetime); err == nil {
			dt = parsedTime
		}
	}

	var links []model.Link
	if item.URL != "" && p.isValidNetworkDriveURL(item.URL) {
		links = append(links, model.Link{
			Type:     p.determineLinkType(item.URL),
			URL:      item.URL,
			Password: item.Password,
		})
	}

	return model.SearchResult{
		UniqueID: fmt.Sprintf("%s-deep-%s", p.Name(), item.ID),
		Title:    item.Title,
		Content:  content,
		Tags:     []string{},
		Links:    links,
		Images:   []string{},
		Channel:  "",
		Datetime: dt,
	}
}

func (p *Pioz3AsyncPlugin) parseDeepSearchServerActionResponse(body []byte) []model.SearchResult {
	var results []model.SearchResult

	text := string(body)

	lines := strings.Split(text, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		if strings.HasPrefix(line, "1:") || strings.Contains(line, `"results"`) {
			jsonPart := line
			if strings.HasPrefix(line, "1:") {
				jsonPart = strings.TrimPrefix(line, "1:")
			}

			var response struct {
				Code    int    `json:"code"`
				Message string `json:"message"`
				Total   int    `json:"total"`
				Results []struct {
					ID        string `json:"id"`
					Title     string `json:"title"`
					URL       string `json:"url"`
					CloudType string `json:"cloud_type"`
					Password  string `json:"password"`
					Datetime  string `json:"datetime"`
				} `json:"results"`
			}

			if err := json.Unmarshal([]byte(jsonPart), &response); err != nil {
				continue
			}

			if response.Code != 0 || len(response.Results) == 0 {
				continue
			}

			for _, item := range response.Results {
				results = append(results, p.convertDeepItem(item.ID, item.Title, item.CloudType, item.Datetime, item.URL, item.Password))
			}

			return results
		}
	}

	return nil
}

func (p *Pioz3AsyncPlugin) convertDeepItem(id, title, cloudType, datetimeStr, linkURL, password string) model.SearchResult {
	content := "source: " + p.getCloudTypeName(cloudType) + " | deep-search"
	if datetimeStr != "" {
		content = content + " | time: " + datetimeStr
	}

	var dt time.Time
	if datetimeStr != "" {
		if parsed, err := time.Parse("2006-01-02T15:04:05Z", datetimeStr); err == nil {
			dt = parsed
		} else if parsed, err := time.Parse("2006-01-02 15:04", datetimeStr); err == nil {
			dt = parsed
		} else if parsed, err := time.Parse("2006-01-02", datetimeStr); err == nil {
			dt = parsed
		}
	}

	var links []model.Link
	if linkURL != "" && p.isValidNetworkDriveURL(linkURL) {
		links = append(links, model.Link{
			Type:     p.determineLinkType(linkURL),
			URL:      linkURL,
			Password: password,
		})
	}

	return model.SearchResult{
		UniqueID: fmt.Sprintf("%s-deep-%s", p.Name(), id),
		Title:    title,
		Content:  content,
		Tags:     []string{},
		Links:    links,
		Images:   []string{},
		Channel:  "",
		Datetime: dt,
	}
}

func (p *Pioz3AsyncPlugin) enhanceDeepSearchResults(client *http.Client, results []model.SearchResult) []model.SearchResult {
	if len(results) == 0 {
		return results
	}

	fmt.Printf("[%s] [增强] 开始增强深度搜索结果，共 %d 个结果\n", p.Name(), len(results))

	if len(results) > MaxEnhancedResults {
		fmt.Printf("[%s] [增强] 警告: 结果数量 %d 超过 MaxEnhancedResults=%d，将处理所有结果\n", p.Name(), len(results), MaxEnhancedResults)
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

			if len(r.Links) > 0 {
				fmt.Printf("[%s] [增强] 结果已有链接: %s (%d个链接)\n", p.Name(), r.Title, len(r.Links))
				mu.Lock()
				enhancedResults = append(enhancedResults, r)
				mu.Unlock()
				return
			}

			p.applyAntiCrawlerDelay()

			if cached, ok := detailCache.Load(r.UniqueID); ok {
				if cachedResult, ok := cached.(model.SearchResult); ok {
					fmt.Printf("[%s] [增强] 命中缓存: %s\n", p.Name(), r.Title)
					mu.Lock()
					enhancedResults = append(enhancedResults, cachedResult)
					mu.Unlock()
					return
				}
			}

			links := p.fetchDeepLinkViaServerAction(client, "quark", r.UniqueID, r.Title, "")
			r.Links = links

			if len(links) > 0 {
				fmt.Printf("[%s] [增强] 成功获取链接: %s -> %d个链接\n", p.Name(), r.Title, len(links))
				for i, link := range links {
					fmt.Printf("[%s] [增强]   链接%d: %s\n", p.Name(), i+1, link.URL)
				}
			} else {
				fmt.Printf("[%s] [增强] 未获取到链接: %s\n", p.Name(), r.Title)
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
	fmt.Printf("[%s] [增强] 深度搜索增强完成: 输入=%d, 输出=%d, 含链接=%d\n", p.Name(), len(results), len(enhancedResults), withLinks)
	return enhancedResults
}

func (p *Pioz3AsyncPlugin) fetchDeepLinkViaServerAction(client *http.Client, adapter, resourceURL, title, password string) []model.Link {
	ctx, cancel := context.WithTimeout(context.Background(), DetailTimeout)
	defer cancel()

	bodyData := []map[string]string{
		{
			"adapter":  adapter,
			"url":      resourceURL,
			"title":    title,
			"password": password,
		},
	}
	bodyBytes, _ := json.Marshal(bodyData)

	req, err := http.NewRequestWithContext(ctx, "POST", SiteBaseURL+"/", bytes.NewReader(bodyBytes))
	if err != nil {
		return nil
	}

	req.Header.Set("Accept", "text/x-component")
	req.Header.Set("Content-Type", "text/plain;charset=UTF-8")
	req.Header.Set("next-action", DeepLinkActionID)
	p.setStealthHeaders(req)
	p.addSessionCookies(req)

	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("[%s] [链接] fetchDeepLinkViaServerAction: 请求失败 err=%v\n", p.Name(), err)
		return nil
	}
	defer resp.Body.Close()

	respBody, err := p.readCompressedBody(resp)
	if err != nil {
		return nil
	}

	respText := string(respBody)
	if len(respText) > 300 {
		respText = respText[:300]
	}
	fmt.Printf("[%s] [链接] fetchDeepLinkViaServerAction: status=%d body=%s\n", p.Name(), resp.StatusCode, respText)

	urlMatch := regexp.MustCompile(`"url"\s*:\s*"([^"]+)"`).FindStringSubmatch(respText)
	if len(urlMatch) > 1 {
		realURL := urlMatch[1]
		if p.isValidNetworkDriveURL(realURL) {
			pwMatch := regexp.MustCompile(`"password"\s*:\s*"([^"]*)"`).FindStringSubmatch(respText)
			pw := ""
			if len(pwMatch) > 1 {
				pw = pwMatch[1]
			}
			fmt.Printf("[%s] [链接] fetchDeepLinkViaServerAction成功: url=%s\n", p.Name(), realURL)
			return []model.Link{{
				Type:     p.determineLinkType(realURL),
				URL:      realURL,
				Password: pw,
			}}
		}
	}

	return nil
}

func (p *Pioz3AsyncPlugin) readCompressedBody(resp *http.Response) ([]byte, error) {
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

func (p *Pioz3AsyncPlugin) setStealthHeaders(req *http.Request) {
	req.Header.Set("User-Agent", p.currentUA())
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,image/apng,*/*;q=0.8")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")
	req.Header.Set("Accept-Encoding", "gzip, deflate")
	req.Header.Set("Connection", "keep-alive")
	req.Header.Set("Cache-Control", "max-age=0")
	req.Header.Set("Pragma", "no-cache")
	if req.URL.Host == "www.pioz.cn" {
		req.Header.Set("Referer", "https://www.pioz.cn/")
	}
}

func (p *Pioz3AsyncPlugin) setAPIHeaders(req *http.Request) {
	req.Header.Set("User-Agent", p.currentUA())
	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")
	req.Header.Set("Accept-Encoding", "gzip, deflate")
	req.Header.Set("Connection", "keep-alive")
	req.Header.Set("Referer", SiteBaseURL+"/")
}

func (p *Pioz3AsyncPlugin) addSessionCookies(req *http.Request) {
	p.sessionMu.RLock()
	defer p.sessionMu.RUnlock()

	for _, cookie := range p.sessionCookies {
		req.AddCookie(cookie)
	}

	if len(p.sessionCookies) == 0 {
		req.AddCookie(&http.Cookie{Name: "first_visit", Value: "1"})
		req.AddCookie(&http.Cookie{Name: "session_id", Value: fmt.Sprintf("%d", time.Now().Unix())})
	}
}

func (p *Pioz3AsyncPlugin) isValidNetworkDriveURL(rawURL string) bool {
	if strings.Contains(rawURL, "javascript:") ||
		strings.Contains(rawURL, "#") ||
		rawURL == "" ||
		(!strings.HasPrefix(rawURL, "http") && !strings.HasPrefix(rawURL, "magnet:") && !strings.HasPrefix(rawURL, "ed2k:")) {
		return false
	}

	return p.determineLinkType(rawURL) != ""
}

func (p *Pioz3AsyncPlugin) determineLinkType(rawURL string) string {
	switch {
	case quarkLinkRegex.MatchString(rawURL):
		return "quark"
	case baiduLinkRegex.MatchString(rawURL):
		return "baidu"
	case aliyunLinkRegex.MatchString(rawURL):
		return "aliyun"
	case ucLinkRegex.MatchString(rawURL):
		return "uc"
	case xunleiLinkRegex.MatchString(rawURL):
		return "xunlei"
	case tianyiLinkRegex.MatchString(rawURL):
		return "tianyi"
	case lanzouLinkRegex.MatchString(rawURL):
		return "lanzou"
	case link115Regex.MatchString(rawURL):
		return "115"
	case mobileLinkRegex.MatchString(rawURL):
		return "mobile"
	case weiyunLinkRegex.MatchString(rawURL):
		return "weiyun"
	case jianguoyunLinkRegex.MatchString(rawURL):
		return "jianguoyun"
	case link123Regex.MatchString(rawURL):
		return "123"
	case pikpakLinkRegex.MatchString(rawURL):
		return "pikpak"
	case magnetLinkRegex.MatchString(rawURL):
		return "magnet"
	case ed2kLinkRegex.MatchString(rawURL):
		return "ed2k"
	default:
		return ""
	}
}

func (p *Pioz3AsyncPlugin) getCloudTypeName(cloudType string) string {
	typeMap := map[string]string{
		"quark":      "quark",
		"baidu":      "baidu",
		"xunlei":     "xunlei",
		"aliyun":     "aliyun",
		"uc":         "uc",
		"lanzou":     "lanzou",
		"115":        "115",
		"mobile":     "mobile",
		"weiyun":     "weiyun",
		"jianguoyun": "jianguoyun",
		"123":        "123",
		"pikpak":     "pikpak",
		"tianyi":     "tianyi",
		"magnet":     "magnet",
		"ed2k":       "ed2k",
	}

	normalized := strings.ToLower(strings.TrimSpace(cloudType))
	if name, ok := typeMap[normalized]; ok {
		return name
	}
	return normalized
}
