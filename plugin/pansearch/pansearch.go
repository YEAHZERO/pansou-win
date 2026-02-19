package pansearch

// ============================================================================
// PanSearch 插件
// 数据源：pansearch.me Next.js API
// 职责：分页抓取搜索结果，去重后转换为统一 SearchResult
// ============================================================================

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

	"pansou/model"
	"pansou/plugin"
	"pansou/util/json"
	"sync/atomic"
)

// 预编译正则与搜索缓存
var (
	buildIdRegex = regexp.MustCompile(`"buildId":"([^"]+)"`)

	nextDataRegex = regexp.MustCompile(`<script id="__NEXT_DATA__" type="application/json">(.*?)</script>`)

	searchResultCache  = sync.Map{}
	lastCacheCleanTime = time.Now()
	cacheTTL           = 1 * time.Hour
)

// init 注册插件并启动缓存清理。
func init() {

	plugin.RegisterGlobalPlugin(NewPanSearchPlugin())

	go startCacheCleaner()
}

// startCacheCleaner 定期清理搜索缓存。
func startCacheCleaner() {

	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for range ticker.C {

		searchResultCache = sync.Map{}
		lastCacheCleanTime = time.Now()
	}
}

// 搜索缓存结构
type cachedResponse struct {
	results   []model.SearchResult
	timestamp time.Time
}

// 常量配置
const (
	WebsiteURL = "https://www.pansearch.me/search"

	BaseURLTemplate = "https://www.pansearch.me/_next/data/%s/search.json"

	DefaultTimeout = 6 * time.Second
	PageSize       = 10
	MaxResults     = 1000
	MaxConcurrent  = 200
	MaxRetries     = 2
	MaxAPIPages    = 100

	MaxIdleConns          = 500
	MaxIdleConnsPerHost   = 200
	MaxConnsPerHost       = 400
	IdleConnTimeout       = 120 * time.Second
	TLSHandshakeTimeout   = 10 * time.Second
	ExpectContinueTimeout = 1 * time.Second
	WriteBufferSize       = 16 * 1024
	ReadBufferSize        = 16 * 1024

	BuildIdCacheDuration = 30
)

// buildId 缓存
var (
	buildIdCache     string
	buildIdCacheTime time.Time
	buildIdMutex     sync.RWMutex
)

// 插件主结构
type PanSearchAsyncPlugin struct {
	*plugin.BaseAsyncPlugin
	timeout       time.Duration
	maxResults    int
	maxConcurrent int
	retries       int
	workerPool    *WorkerPool
}

// 工作池结构
type WorkerPool struct {
	tasks   chan Task
	results chan TaskResult
	errors  chan error
	wg      sync.WaitGroup
	closed  atomic.Bool
	mu      sync.Mutex
}

// 工作任务
type Task struct {
	keyword string
	offset  int
	baseURL string
}

// 任务结果
type TaskResult struct {
	offset  int
	results []PanSearchItem
}

// NewWorkerPool 创建工作池。
func NewWorkerPool(size int) *WorkerPool {
	return &WorkerPool{
		tasks:   make(chan Task, size*3),
		results: make(chan TaskResult, size*3),
		errors:  make(chan error, size*3),
	}
}

// Start 启动 worker 消费任务。
func (wp *WorkerPool) Start(ctx context.Context, handler func(ctx context.Context, task Task) (TaskResult, error)) {
	for i := 0; i < cap(wp.tasks); i++ {
		wp.wg.Add(1)
		go func() {
			defer wp.wg.Done()
			for {
				select {
				case task, ok := <-wp.tasks:
					if !ok {
						return
					}

					result, err := handler(ctx, task)
					if err != nil {
						select {
						case wp.errors <- err:

						default:

							fmt.Printf("无法发送错误: %v\n", err)
						}
					} else {
						select {
						case wp.results <- result:

						default:

							fmt.Printf("无法发送结果\n")
						}
					}

				case <-ctx.Done():
					return
				}
			}
		}()
	}
}

// Submit 提交任务到工作池。
func (wp *WorkerPool) Submit(task Task) bool {
	wp.mu.Lock()
	defer wp.mu.Unlock()

	if wp.closed.Load() {
		return false
	}

	select {
	case wp.tasks <- task:
		return true
	default:

		return false
	}
}

// Close 关闭工作池并回收资源。
func (wp *WorkerPool) Close() {
	wp.mu.Lock()
	if !wp.closed.Load() {
		wp.closed.Store(true)
		close(wp.tasks)
	}
	wp.mu.Unlock()

	wp.wg.Wait()

	wp.mu.Lock()
	defer wp.mu.Unlock()

	select {
	case _, ok := <-wp.results:
		if ok {
			close(wp.results)
		}
	default:
		close(wp.results)
	}

	select {
	case _, ok := <-wp.errors:
		if ok {
			close(wp.errors)
		}
	default:
		close(wp.errors)
	}
}

// NewPanSearchPlugin 创建插件实例。
func NewPanSearchPlugin() *PanSearchAsyncPlugin {
	timeout := DefaultTimeout
	maxConcurrent := MaxConcurrent

	p := &PanSearchAsyncPlugin{
		BaseAsyncPlugin: plugin.NewBaseAsyncPlugin("pansearch", 1),
		timeout:         timeout,
		maxResults:      MaxResults,
		maxConcurrent:   maxConcurrent,
		retries:         MaxRetries,
		workerPool:      NewWorkerPool(maxConcurrent),
	}

	go func() {
		_, err := p.getBuildId()
		if err != nil {

		}
	}()

	go p.startBuildIdUpdater()

	return p
}

// startBuildIdUpdater 后台定时刷新 buildId。
func (p *PanSearchAsyncPlugin) startBuildIdUpdater() {

	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		p.updateBuildId()
	}
}

// updateBuildId 主动刷新 buildId 缓存。
func (p *PanSearchAsyncPlugin) updateBuildId() {

	ctx, cancel := context.WithTimeout(context.Background(), p.timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", WebsiteURL, nil)
	if err != nil {

		return
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")
	req.Header.Set("Connection", "keep-alive")
	req.Header.Set("Upgrade-Insecure-Requests", "1")
	req.Header.Set("Cache-Control", "max-age=0")

	resp, err := p.GetClient().Do(req)
	if err != nil {

		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		fmt.Printf("获取buildId时服务器返回非200状态码: %d\n", resp.StatusCode)
		return
	}

	var bodyBuilder strings.Builder
	_, err = io.Copy(&bodyBuilder, resp.Body)
	if err != nil {

		return
	}
	body := bodyBuilder.String()

	newBuildId := extractBuildId(body)
	if newBuildId == "" {
		fmt.Println("未能从响应中提取 buildId")
		return
	}

	buildIdMutex.Lock()
	defer buildIdMutex.Unlock()

	if newBuildId != "" && newBuildId != buildIdCache {
		buildIdCache = newBuildId
		buildIdCacheTime = time.Now()
		fmt.Printf("成功更新 buildId: %s\n", newBuildId)
	}
}

// extractBuildId 从页面源码提取 buildId。
func extractBuildId(body string) string {

	matches := buildIdRegex.FindStringSubmatch(body)

	if len(matches) >= 2 {
		return matches[1]
	}

	scriptMatches := nextDataRegex.FindStringSubmatch(body)

	if len(scriptMatches) >= 2 {
		var nextData map[string]interface{}
		if err := json.Unmarshal([]byte(scriptMatches[1]), &nextData); err == nil {
			if buildId, ok := nextData["buildId"].(string); ok && buildId != "" {
				return buildId
			}
		}
	}

	return ""
}

// Name 返回插件名。
func (p *PanSearchAsyncPlugin) Name() string {
	return "pansearch"
}

// Priority 返回插件优先级。
func (p *PanSearchAsyncPlugin) Priority() int {
	return 1
}

// getBuildId 获取可用 buildId（带缓存）。
func (p *PanSearchAsyncPlugin) getBuildId() (string, error) {

	buildIdMutex.RLock()
	if buildIdCache != "" && time.Since(buildIdCacheTime) < BuildIdCacheDuration*time.Minute {
		defer buildIdMutex.RUnlock()
		return buildIdCache, nil
	}
	buildIdMutex.RUnlock()

	buildIdMutex.Lock()
	defer buildIdMutex.Unlock()

	if buildIdCache != "" && time.Since(buildIdCacheTime) < BuildIdCacheDuration*time.Minute {
		return buildIdCache, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), p.timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", WebsiteURL, nil)
	if err != nil {

		if buildIdCache != "" {

			return buildIdCache, nil
		}
		return "", fmt.Errorf("创建请求失败: %w", err)
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")
	req.Header.Set("Connection", "keep-alive")
	req.Header.Set("Upgrade-Insecure-Requests", "1")
	req.Header.Set("Cache-Control", "max-age=0")

	var resp *http.Response
	var respErr error

	for retry := 0; retry <= p.retries; retry++ {
		if retry > 0 {

			backoffTime := time.Duration(1<<uint(retry-1)) * 100 * time.Millisecond
			time.Sleep(backoffTime)
		}

		resp, respErr = p.GetClient().Do(req)
		if respErr == nil && resp.StatusCode == 200 {
			break
		}

		if resp != nil {
			resp.Body.Close()
		}
	}

	if respErr != nil || resp == nil {
		if buildIdCache != "" {

			return buildIdCache, nil
		}
		return "", fmt.Errorf("请求失败: %w", respErr)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {

		if buildIdCache != "" {
			fmt.Printf("获取buildId时服务器返回非200状态码: %d，使用旧的buildId\n", resp.StatusCode)
			return buildIdCache, nil
		}
		return "", fmt.Errorf("获取buildId时服务器返回非200状态码: %d", resp.StatusCode)
	}

	var bodyBuilder strings.Builder
	_, err = io.Copy(&bodyBuilder, resp.Body)
	if err != nil {

		if buildIdCache != "" {

			return buildIdCache, nil
		}
		return "", fmt.Errorf("读取响应失败: %w", err)
	}
	body := bodyBuilder.String()

	buildId := extractBuildId(body)

	if buildId == "" {
		if buildIdCache != "" {

			return buildIdCache, nil
		}
		return "", fmt.Errorf("未找到buildId")
	}

	buildIdCache = buildId
	buildIdCacheTime = time.Now()

	return buildId, nil
}

// getBaseURL 组合 API 基础地址。
func (p *PanSearchAsyncPlugin) getBaseURL(client *http.Client) (string, error) {
	buildId, err := p.getBuildId()
	if err != nil {
		return "", err
	}

	return fmt.Sprintf(BaseURLTemplate, buildId), nil
}

// Search 兼容基础搜索接口。
func (p *PanSearchAsyncPlugin) Search(keyword string, ext map[string]interface{}) ([]model.SearchResult, error) {
	result, err := p.SearchWithResult(keyword, ext)
	if err != nil {
		return nil, err
	}
	return result.Results, nil
}

// SearchWithResult 走框架异步搜索入口。
func (p *PanSearchAsyncPlugin) SearchWithResult(keyword string, ext map[string]interface{}) (model.PluginSearchResult, error) {
	return p.AsyncSearchWithResult(keyword, p.doSearch, p.MainCacheKey, ext)
}

// doSearch 主流程：抓取首页、并发抓取后续页、去重与转换。
func (p *PanSearchAsyncPlugin) doSearch(client *http.Client, keyword string, ext map[string]interface{}) ([]model.SearchResult, error) {

	fmt.Printf("[%s] %s\n", p.Name(), "www.pansearch.me")

	baseURL, err := p.getBaseURL(client)
	if err != nil {
		return nil, fmt.Errorf("获取API基础URL失败: %w", err)
	}

	firstPageResults, total, err := p.fetchFirstPage(keyword, baseURL, client)
	if err != nil {

		if strings.Contains(err.Error(), "404") || strings.Contains(err.Error(), "Not Found") {
			fmt.Println("检测到404错误，buildId可能已过期，尝试强制刷新")

			buildIdMutex.Lock()
			buildIdCache = ""
			buildIdCacheTime = time.Time{}
			buildIdMutex.Unlock()

			baseURL, err = p.getBaseURL(client)
			if err != nil {
				return nil, fmt.Errorf("刷新buildId失败: %w", err)
			}

			firstPageResults, total, err = p.fetchFirstPage(keyword, baseURL, client)
			if err != nil {
				return nil, fmt.Errorf("刷新buildId后获取首页仍然失败: %w", err)
			}

			go p.updateBuildId()
		} else {
			return nil, fmt.Errorf("获取首页失败: %w", err)
		}
	}

	allResults := firstPageResults

	remainingResults := min(total-PageSize, p.maxResults-PageSize)
	if remainingResults <= 0 {
		results := p.convertResults(allResults, keyword)

		searchResultCache.Store(keyword, cachedResponse{
			results:   results,
			timestamp: time.Now(),
		})

		return results, nil
	}

	neededPages := min((remainingResults+PageSize-1)/PageSize, MaxAPIPages-1)

	if neededPages <= 0 {
		results := p.convertResults(allResults, keyword)

		searchResultCache.Store(keyword, cachedResponse{
			results:   results,
			timestamp: time.Now(),
		})

		return results, nil
	}

	actualConcurrent := min(neededPages, p.maxConcurrent)

	p.workerPool = NewWorkerPool(actualConcurrent)

	ctx, cancel := context.WithTimeout(context.Background(), p.timeout*2)
	defer cancel()

	needRefreshBuildId := &atomic.Bool{}

	p.workerPool.Start(ctx, func(ctx context.Context, task Task) (TaskResult, error) {
		var pageResults []PanSearchItem
		var err error

		for retry := 0; retry <= p.retries; retry++ {

			if needRefreshBuildId.Load() {
				time.Sleep(100 * time.Millisecond)
				continue
			}

			pageResults, err = p.fetchPage(task.keyword, task.offset, task.baseURL)
			if err == nil {
				break
			}

			if strings.Contains(err.Error(), "404") || strings.Contains(err.Error(), "Not Found") {

				if !needRefreshBuildId.Load() {
					needRefreshBuildId.Store(true)

					go func() {
						buildIdMutex.Lock()
						buildIdCache = ""
						buildIdCacheTime = time.Time{}
						buildIdMutex.Unlock()

						newBuildId, err := p.getBuildId()
						if err == nil && newBuildId != "" {

							task.baseURL = fmt.Sprintf(BaseURLTemplate, newBuildId)
							fmt.Printf("成功刷新buildId: %s\n", newBuildId)
						}

						needRefreshBuildId.Store(false)
					}()
				}

				for i := 0; i < 10 && needRefreshBuildId.Load(); i++ {
					time.Sleep(100 * time.Millisecond)
				}

				if needRefreshBuildId.Load() {
					return TaskResult{}, fmt.Errorf("404错误，buildId可能已过期: %w", err)
				}

				continue
			}

			if retry < p.retries {

				select {
				case <-time.After(time.Duration(1<<retry) * 100 * time.Millisecond):

				case <-ctx.Done():
					return TaskResult{}, ctx.Err()
				}
			}
		}

		if err != nil {
			return TaskResult{}, fmt.Errorf("获取偏移量 %d 的结果失败: %w", task.offset, err)
		}

		return TaskResult{offset: task.offset, results: pageResults}, nil
	})

	submittedTasks := 0

	batchSize := (neededPages + 1) / 2
	if batchSize < 1 {
		batchSize = neededPages
	}

	for i := 0; i < neededPages; i += batchSize {

		select {
		case <-ctx.Done():

			goto CollectResults
		default:

		}

		end := i + batchSize
		if end > neededPages {
			end = neededPages
		}

		for j := i; j < end; j++ {
			offset := PageSize + j*PageSize
			if offset < p.maxResults {
				task := Task{
					keyword: keyword,
					offset:  offset,
					baseURL: baseURL,
				}

				if !p.workerPool.Submit(task) {
					fmt.Printf("无法提交任务，工作池可能已关闭\n")
					goto CollectResults
				}

				submittedTasks++
			}
		}

		if batchSize < neededPages && end < neededPages {
			select {
			case <-time.After(50 * time.Millisecond):

			case <-ctx.Done():

				goto CollectResults
			}
		}
	}

CollectResults:

	go p.workerPool.Close()

	resultCount := 0
	errorCount := 0
	var lastError error

	for resultCount+errorCount < submittedTasks {
		select {
		case result, ok := <-p.workerPool.results:
			if !ok {

				goto ProcessResults
			}
			allResults = append(allResults, result.results...)
			resultCount++

		case err, ok := <-p.workerPool.errors:
			if !ok {

				goto ProcessResults
			}
			errorCount++
			lastError = err

		case <-ctx.Done():

			results := p.convertResults(allResults, keyword)

			searchResultCache.Store(keyword, cachedResponse{
				results:   results,
				timestamp: time.Now(),
			})

			return results, fmt.Errorf("搜索超时: %w", ctx.Err())
		}
	}

ProcessResults:

	if submittedTasks > 0 && errorCount == submittedTasks && len(allResults) == len(firstPageResults) {
		results := p.convertResults(allResults, keyword)

		searchResultCache.Store(keyword, cachedResponse{
			results:   results,
			timestamp: time.Now(),
		})

		return results, fmt.Errorf("所有后续页面请求失败: %v", lastError)
	}

	uniqueResults := p.deduplicateItems(allResults)
	results := p.convertResults(uniqueResults, keyword)

	searchResultCache.Store(keyword, cachedResponse{
		results:   results,
		timestamp: time.Now(),
	})

	return results, nil
}

// fetchFirstPage 获取第一页并返回总条数。
func (p *PanSearchAsyncPlugin) fetchFirstPage(keyword string, baseURL string, client *http.Client) ([]PanSearchItem, int, error) {

	reqURL := fmt.Sprintf("%s?keyword=%s&offset=0", baseURL, url.QueryEscape(keyword))

	ctx, cancel := context.WithTimeout(context.Background(), p.timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return nil, 0, fmt.Errorf("创建请求失败: %w", err)
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36")
	req.Header.Set("Referer", "https://www.pansearch.me/")
	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")
	req.Header.Set("Connection", "keep-alive")
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("Pragma", "no-cache")

	resp, err := client.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 404 {
		return nil, 0, fmt.Errorf("404 Not Found，buildId可能已过期")
	}

	if resp.StatusCode != 200 {
		return nil, 0, fmt.Errorf("服务器返回非200状态码: %d", resp.StatusCode)
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, 0, fmt.Errorf("读取响应失败: %w", err)
	}

	var apiResp PanSearchResponse
	if err := json.Unmarshal(respBody, &apiResp); err != nil {
		return nil, 0, fmt.Errorf("解析响应失败: %w", err)
	}

	total := apiResp.PageProps.Data.Total
	items := apiResp.PageProps.Data.Data

	return items, total, nil
}

// fetchPage 按偏移量获取分页数据。
func (p *PanSearchAsyncPlugin) fetchPage(keyword string, offset int, baseURL string) ([]PanSearchItem, error) {

	reqURL := fmt.Sprintf("%s?keyword=%s&offset=%d", baseURL, url.QueryEscape(keyword), offset)

	ctx, cancel := context.WithTimeout(context.Background(), p.timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36")
	req.Header.Set("Referer", "https://www.pansearch.me/")
	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")
	req.Header.Set("Connection", "keep-alive")
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("Pragma", "no-cache")

	resp, err := p.GetClient().Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 404 {
		return nil, fmt.Errorf("404 Not Found，buildId可能已过期")
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("服务器返回非200状态码: %d", resp.StatusCode)
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %w", err)
	}

	var apiResp PanSearchResponse
	if err := json.Unmarshal(respBody, &apiResp); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}

	return apiResp.PageProps.Data.Data, nil
}

// deduplicateItems 按标题与内容去重。
func (p *PanSearchAsyncPlugin) deduplicateItems(items []PanSearchItem) []PanSearchItem {

	uniqueMap := make(map[int]PanSearchItem)

	for _, item := range items {
		uniqueMap[item.ID] = item
	}

	result := make([]PanSearchItem, 0, len(uniqueMap))
	for _, item := range uniqueMap {
		result = append(result, item)
	}

	return result
}

// convertResults 转换为统一 SearchResult。
func (p *PanSearchAsyncPlugin) convertResults(items []PanSearchItem, keyword string) []model.SearchResult {
	results := make([]model.SearchResult, 0, len(items))

	for _, item := range items {

		linkInfo := extractLinkAndPassword(item.Content)

		linkType := item.Pan

		if linkType == "aliyundrive" {
			linkType = "aliyun"
		}

		link := model.Link{
			URL:      linkInfo.URL,
			Type:     linkType,
			Password: linkInfo.Password,
		}

		uniqueID := fmt.Sprintf("pansearch-%d", item.ID)

		var datetime time.Time
		if item.Time != "" {

			parsedTime, err := time.Parse(time.RFC3339, item.Time)
			if err == nil {
				datetime = parsedTime
			}
		}

		if datetime.IsZero() {
			datetime = time.Time{}
		}

		result := model.SearchResult{
			UniqueID: uniqueID,
			Title:    extractTitle(item.Content, keyword),
			Content:  item.Content,
			Datetime: datetime,
			Links:    []model.Link{link},
		}

		results = append(results, result)
	}

	return results
}

type LinkInfo struct {
	URL      string
	Password string
}

// extractLinkAndPassword 从内容中提取链接与提取码。
func extractLinkAndPassword(content string) LinkInfo {

	linkInfo := LinkInfo{}

	linkStartIndex := strings.Index(content, "href=\"")
	if linkStartIndex != -1 {
		linkStartIndex += 6
		linkEndIndex := strings.Index(content[linkStartIndex:], "\"")
		if linkEndIndex != -1 {
			linkInfo.URL = content[linkStartIndex : linkStartIndex+linkEndIndex]
		}
	}

	pwdIndex := strings.Index(content, "?pwd=")
	if pwdIndex != -1 {
		pwdStartIndex := pwdIndex + 5
		pwdEndIndex := strings.Index(content[pwdStartIndex:], "\"")
		if pwdEndIndex != -1 {
			linkInfo.Password = content[pwdStartIndex : pwdStartIndex+pwdEndIndex]
		} else {

			pwdEndIndex = strings.Index(content[pwdStartIndex:], "#")
			if pwdEndIndex != -1 {
				linkInfo.Password = content[pwdStartIndex : pwdStartIndex+pwdEndIndex]
			} else {

				linkInfo.Password = content[pwdStartIndex:]
			}
		}
	}

	return linkInfo
}

// extractTitle 从内容中提取标题。
func extractTitle(content string, keyword string) string {

	titlePrefix := "名称："
	titleStartIndex := strings.Index(content, titlePrefix)
	if titleStartIndex == -1 {
		return keyword
	}

	titleStartIndex += len(titlePrefix)
	titleEndIndex := strings.Index(content[titleStartIndex:], "\n")
	if titleEndIndex == -1 {
		return cleanHTML(content[titleStartIndex:])
	}

	return cleanHTML(content[titleStartIndex : titleStartIndex+titleEndIndex])
}

// cleanHTML 清理 HTML 标签与多余空白。
func cleanHTML(html string) string {

	replacements := map[string]string{
		"<span class='highlight-keyword'>": "",
		"</span>":                          "",
		"<a class=\"resource-link\" target=\"_blank\" href=\"": "",
		"</a>": "",
		"<br>": "\n",
		"<p>":  "",
		"</p>": "\n",
	}

	result := html
	for tag, replacement := range replacements {
		result = strings.Replace(result, tag, replacement, -1)
	}

	for {
		startIndex := strings.Index(result, "<")
		if startIndex == -1 {
			break
		}

		endIndex := strings.Index(result[startIndex:], ">")
		if endIndex == -1 {
			break
		}

		result = result[:startIndex] + result[startIndex+endIndex+1:]
	}

	return strings.TrimSpace(result)
}

// min 返回两个整数中的较小值。
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

type PanSearchResponse struct {
	PageProps struct {
		Data struct {
			Total int             `json:"total"`
			Data  []PanSearchItem `json:"data"`
			Time  int             `json:"time"`
		} `json:"data"`
		Limit    int  `json:"limit"`
		IsMobile bool `json:"isMobile"`
	} `json:"pageProps"`
	NSSP bool `json:"__N_SSP"`
}

type PanSearchItem struct {
	ID      int    `json:"id"`
	Content string `json:"content"`
	Pan     string `json:"pan"`
	Image   string `json:"image"`
	Time    string `json:"time"`
}
