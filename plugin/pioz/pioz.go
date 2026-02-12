package pioz

import (
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
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

// 辅助函数
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// 获取字符串构建器
func getStringBuilder() *strings.Builder {
	sb := stringBuilderPool.Get().(*strings.Builder)
	sb.Reset()
	return sb
}

// 归还字符串构建器
func putStringBuilder(sb *strings.Builder) {
	sb.Reset()
	stringBuilderPool.Put(sb)
}

// 获取链接对象
func (p *PiozPlugin) getLink() *model.Link {
	return p.linkPool.Get().(*model.Link)
}

// 归还链接对象
func (p *PiozPlugin) putLink(link *model.Link) {
	link.Type = ""
	link.URL = ""
	link.Password = ""
	p.linkPool.Put(link)
}

// 获取搜索结果对象
func (p *PiozPlugin) getSearchResult() *model.SearchResult {
	return p.searchResultPool.Get().(*model.SearchResult)
}

// 归还搜索结果对象
func (p *PiozPlugin) putSearchResult(result *model.SearchResult) {
	result.UniqueID = ""
	result.Title = ""
	result.Content = ""
	result.Datetime = time.Time{}
	result.Links = nil
	result.Channel = ""
	p.searchResultPool.Put(result)
}

// ==================== 缓存管理 ====================

// manageSearchCache 管理搜索缓存大小
func manageSearchCache() {
	cacheStats.Lock()
	defer cacheStats.Unlock()

	if cacheStats.searchCacheSize > MaxSearchCacheSize {
		// 清理旧缓存
		oldKeys := []interface{}{}
		searchCache.Range(func(key, value interface{}) bool {
			if cachedResp, ok := value.(cachedResponse); ok {
				// 清理超过30分钟的缓存
				if time.Since(cachedResp.timestamp) > CacheTTL {
					oldKeys = append(oldKeys, key)
				}
			}
			return true
		})

		// 删除旧缓存
		for _, key := range oldKeys {
			searchCache.Delete(key)
			cacheStats.searchCacheSize--
		}
	}
}

// manageDetailCache 管理详情缓存大小
func manageDetailCache() {
	cacheStats.Lock()
	defer cacheStats.Unlock()

	if cacheStats.detailCacheSize > MaxDetailCacheSize {
		// 清理旧缓存
		oldKeys := []interface{}{}
		detailCache.Range(func(key, value interface{}) bool {
			// 简单清理，删除一半的缓存
			if len(oldKeys) < cacheStats.detailCacheSize/2 {
				oldKeys = append(oldKeys, key)
			}
			return true
		})

		// 删除旧缓存
		for _, key := range oldKeys {
			detailCache.Delete(key)
			cacheStats.detailCacheSize--
		}
	}
}

// manageTransferCache 管理传输缓存大小
func manageTransferCache() {
	cacheStats.Lock()
	defer cacheStats.Unlock()

	if cacheStats.transferCacheSize > MaxTransferCacheSize {
		// 清理旧缓存
		oldKeys := []interface{}{}
		transferCache.Range(func(key, value interface{}) bool {
			// 简单清理，删除一半的缓存
			if len(oldKeys) < cacheStats.transferCacheSize/2 {
				oldKeys = append(oldKeys, key)
			}
			return true
		})

		// 删除旧缓存
		for _, key := range oldKeys {
			transferCache.Delete(key)
			cacheStats.transferCacheSize--
		}
	}
}

const (
	// 超时时间配置
	DefaultTimeout = 15 * time.Second // 搜索超时
	DetailTimeout  = 12 * time.Second // 详情页超时
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
	RequestDelayMin = 800 * time.Millisecond  // 最小请求间隔
	RequestDelayMax = 2500 * time.Millisecond // 最大请求间隔
	RetryCount      = 2                       // 重试次数

	// 会话管理
	SessionTimeout = 30 * time.Minute // 会话超时时间
)

// 预编译的正则表达式（支持16种网盘链接）
var (
	quarkLinkRegex      = regexp.MustCompile(`(?:https?:)?//pan\.quark\.cn/s/[0-9a-zA-Z]+`)
	baiduLinkRegex      = regexp.MustCompile(`(?:https?:)?//pan\.baidu\.com/s/[0-9a-zA-Z_\-]+(?:\?pwd=[0-9a-zA-Z]+)?`)
	aliyunLinkRegex     = regexp.MustCompile(`(?:https?:)?//(?:www\.)?(?:aliyundrive\.com|alipan\.com)/s/[0-9a-zA-Z]+`)
	ucLinkRegex         = regexp.MustCompile(`(?:https?:)?//drive\.uc\.cn/s/[0-9a-zA-Z]+(?:\?[^"'\s]*)?`)
	xunleiLinkRegex     = regexp.MustCompile(`(?:https?:)?//pan\.xunlei\.com/s/[0-9a-zA-Z_\-]+(?:\?pwd=[0-9a-zA-Z]+)?`)
	tianyiLinkRegex     = regexp.MustCompile(`(?:https?:)?//cloud\.189\.cn/t/[0-9a-zA-Z]+`)
	lanzouLinkRegex     = regexp.MustCompile(`(?:https?:)?//(?:www\.)?(?:lanzou[uixys]*|lan[zs]o[ux])\.(?:com|net|org)/[0-9a-zA-Z]+`)
	link115Regex        = regexp.MustCompile(`(?:https?:)?//115\.com/s/[0-9a-zA-Z]+`)
	mobileLinkRegex     = regexp.MustCompile(`(?:https?:)?//caiyun\.feixin\.10086\.cn/[0-9a-zA-Z]+`)
	weiyunLinkRegex     = regexp.MustCompile(`(?:https?:)?//share\.weiyun\.com/[0-9a-zA-Z]+`)
	jianguoyunLinkRegex = regexp.MustCompile(`(?:https?:)?//(?:www\.)?jianguoyun\.com/p/[0-9a-zA-Z]+`)
	link123Regex        = regexp.MustCompile(`(?:https?:)?//123pan\.com/s/[0-9a-zA-Z]+`)
	pikpakLinkRegex     = regexp.MustCompile(`(?:https?:)?//mypikpak\.com/s/[0-9a-zA-Z]+`)
	magnetLinkRegex     = regexp.MustCompile(`magnet:\?xt=urn:btih:[0-9a-fA-F]{40}`)
	ed2kLinkRegex       = regexp.MustCompile(`ed2k://\|file\|.+\|\d+\|[0-9a-fA-F]{32}\|/`)

	// 密码提取正则
	passwordRegex    = regexp.MustCompile(`(?i)(?:提取码|密码|pwd|码)[：:]\s*([a-zA-Z0-9]{4})`)
	urlPasswordRegex = regexp.MustCompile(`(?i)\?pwd=([0-9a-zA-Z]+)`)

	// 详情页ID提取
	detailIDRegex = regexp.MustCompile(`/detail/(\d+)`)

	// 反爬检测正则
	antiCrawlerRegex = regexp.MustCompile(`禁止使用开发者工具|偷|反爬虫|防爬`)
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

	// 缓存统计
	cacheStats = struct {
		sync.RWMutex
		searchCacheSize   int
		detailCacheSize   int
		transferCacheSize int
	}{}

	// 缓存大小限制
	MaxSearchCacheSize   = 100
	MaxDetailCacheSize   = 200
	MaxTransferCacheSize = 300
)

// 性能统计
var (
	// 基本统计
	searchRequests    int64 = 0
	detailRequests    int64 = 0
	cacheHits         int64 = 0
	cacheMisses       int64 = 0
	antiCrawlerBlocks int64 = 0

	// 时间统计
	totalSearchTime      int64 = 0
	totalDetailTime      int64 = 0
	totalAPIRequestTime  int64 = 0
	totalHTMLRequestTime int64 = 0

	// 错误统计
	errorCount     int64 = 0
	apiErrorCount  int64 = 0
	htmlErrorCount int64 = 0

	// 缓存统计
	searchCacheHits   int64 = 0
	detailCacheHits   int64 = 0
	transferCacheHits int64 = 0

	// 反爬统计
	requestDelayCount int64 = 0
	totalDelayTime    int64 = 0
	userAgentChanges  int64 = 0

	// 网络统计
	successRequests int64 = 0
	failedRequests  int64 = 0

	// 性能统计互斥锁
	statsMutex sync.RWMutex
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
	// 对象池减少内存分配
	linkPool         sync.Pool
	searchResultPool sync.Pool
}

// 字符串构建器池
var (
	stringBuilderPool = sync.Pool{
		New: func() interface{} {
			return &strings.Builder{}
		},
	}
)

// ==================== 插件初始化 ====================

// init 注册插件
func init() {
	plugin.RegisterGlobalPlugin(NewPiozPlugin())
}

// NewPiozPlugin 创建新的 Pioz 插件实例
func NewPiozPlugin() *PiozPlugin {
	// 创建优化的 HTTP 客户端
	transport := &http.Transport{
		MaxIdleConns:          MaxIdleConns,
		MaxIdleConnsPerHost:   MaxIdleConnsPerHost,
		MaxConnsPerHost:       MaxConnsPerHost,
		IdleConnTimeout:       IdleConnTimeout,
		DisableKeepAlives:     false,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 2 * time.Second,
		ResponseHeaderTimeout: 10 * time.Second,
	}

	client := &http.Client{
		Transport: transport,
		Timeout:   DefaultTimeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			// 限制重定向次数
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
	if randomIndex < 0 {
		randomIndex = -randomIndex
	}

	// 创建基础插件，设置更高的优先级
	basePlugin := plugin.NewBaseAsyncPlugin("pioz", 2)

	p := &PiozPlugin{
		BaseAsyncPlugin:  basePlugin,
		optimizedClient:  client,
		userAgents:       userAgents,
		currentUserAgent: userAgents[randomIndex],
	}

	// 初始化对象池
	p.linkPool = sync.Pool{
		New: func() interface{} {
			return &model.Link{}
		},
	}

	// 初始化搜索结果对象池
	p.searchResultPool = sync.Pool{
		New: func() interface{} {
			return &model.SearchResult{}
		},
	}

	// 启动性能监控
	p.startPerformanceMonitor()

	return p
}

// ==================== 搜索接口 ====================

// Search 同步搜索接口
func (p *PiozPlugin) Search(keyword string, ext map[string]interface{}) ([]model.SearchResult, error) {
	// 直接调用 searchImpl 方法，绕过异步超时限制
	return p.searchImpl(p.optimizedClient, keyword, ext)
}

// SearchWithResult 带结果统计的搜索接口
func (p *PiozPlugin) SearchWithResult(keyword string, ext map[string]interface{}) (model.PluginSearchResult, error) {
	// 直接调用 searchImpl 方法，绕过异步超时限制
	results, err := p.searchImpl(p.optimizedClient, keyword, ext)
	if err != nil {
		return model.PluginSearchResult{
			Results: []model.SearchResult{},
			IsFinal: true,
		}, err
	}
	return model.PluginSearchResult{
		Results: results,
		IsFinal: true,
	}, nil
}

// ==================== 搜索实现 ====================

// searchImpl 实现搜索逻辑（多策略搜索）
func (p *PiozPlugin) searchImpl(client *http.Client, keyword string, ext map[string]interface{}) ([]model.SearchResult, error) {
	fmt.Printf("[%s] 开始搜索，keyword='%s'\n", p.Name(), keyword)

	// 性能统计
	start := time.Now()
	atomic.AddInt64(&searchRequests, 1)
	defer func() {
		duration := time.Since(start).Nanoseconds()
		atomic.AddInt64(&totalSearchTime, duration)
		fmt.Printf("[%s] 搜索完成，耗时: %.2f秒\n", p.Name(), float64(duration)/float64(time.Second))
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
	fmt.Printf("[%s] 缓存未命中，开始执行搜索策略\n", p.Name())

	// 使用优化的客户端
	if p.optimizedClient != nil {
		client = p.optimizedClient
		fmt.Printf("[%s] 使用优化的HTTP客户端\n", p.Name())
	}

	// 策略1：深度搜索API（首选）
	fmt.Printf("[%s] 执行策略1：深度搜索API\n", p.Name())
	results, err := p.performDeepSearch(client, keyword)
	if err == nil && len(results) > 0 {
		fmt.Printf("[%s] ✅ 策略1成功：深度搜索API返回 %d 个结果\n", p.Name(), len(results))

		// 检查结果是否已经包含链接
		hasLinks := false
		for _, result := range results {
			if len(result.Links) > 0 {
				hasLinks = true
				break
			}
		}

		// 如果结果已经包含链接，就跳过增强步骤，但需要过滤无效链接
		var enhancedResults []model.SearchResult
		if hasLinks {
			fmt.Printf("[%s] 结果已包含链接，跳过增强步骤，但过滤无效链接\n", p.Name())
			// 过滤掉无效的夸克网盘链接（只保留/s/链接）
			for i := range results {
				var validLinks []model.Link
				for _, link := range results[i].Links {
					if link.Type == "quark" && quarkLinkRegex.MatchString(link.URL) {
						validLinks = append(validLinks, link)
						fmt.Printf("[%s] 保留有效的夸克网盘链接: %s\n", p.Name(), link.URL)
					} else if link.Type != "quark" {
						validLinks = append(validLinks, link)
					} else {
						fmt.Printf("[%s] 过滤掉无效的夸克网盘链接: %s\n", p.Name(), link.URL)
					}
				}
				results[i].Links = validLinks
			}
			enhancedResults = results
		} else {
			// 同步执行结果增强，确保返回真正的网盘链接
			enhancedResults = p.enhanceWithDetails(client, results)
			fmt.Printf("[%s] 结果增强完成，增强后结果数: %d\n", p.Name(), len(enhancedResults))
		}

		// 管理缓存大小
		manageSearchCache()

		// 缓存增强后的结果
		searchCache.Store(cacheKey, cachedResponse{
			results:   enhancedResults,
			timestamp: time.Now(),
		})

		// 更新缓存统计
		cacheStats.Lock()
		cacheStats.searchCacheSize++
		cacheStats.Unlock()

		fmt.Printf("[%s] 结果已缓存，缓存键: %s\n", p.Name(), cacheKey)

		return enhancedResults, nil
	}
	fmt.Printf("[%s] ⚠️ 策略1失败：%v\n", p.Name(), err)

	// 策略2：普通搜索页面（备用）
	fmt.Printf("[%s] 执行策略2：普通HTML搜索\n", p.Name())
	results, err = p.performRegularSearch(client, keyword)
	if err == nil && len(results) > 0 {
		fmt.Printf("[%s] ✅ 策略2成功：HTML搜索返回 %d 个结果\n", p.Name(), len(results))

		// 检查结果是否已经包含链接
		hasLinks := false
		for _, result := range results {
			if len(result.Links) > 0 {
				hasLinks = true
				break
			}
		}

		// 如果结果已经包含链接，就跳过增强步骤，但需要过滤无效链接
		var enhancedResults []model.SearchResult
		if hasLinks {
			fmt.Printf("[%s] 结果已包含链接，跳过增强步骤，但过滤无效链接\n", p.Name())
			// 过滤掉无效的夸克网盘链接（只保留/s/链接）
			for i := range results {
				var validLinks []model.Link
				for _, link := range results[i].Links {
					if link.Type == "quark" && quarkLinkRegex.MatchString(link.URL) {
						validLinks = append(validLinks, link)
						fmt.Printf("[%s] 保留有效的夸克网盘链接: %s\n", p.Name(), link.URL)
					} else if link.Type != "quark" {
						validLinks = append(validLinks, link)
					} else {
						fmt.Printf("[%s] 过滤掉无效的夸克网盘链接: %s\n", p.Name(), link.URL)
					}
				}
				results[i].Links = validLinks
			}
			enhancedResults = results
		} else {
			// 同步执行结果增强，确保返回真正的网盘链接
			enhancedResults = p.enhanceWithDetails(client, results)
			fmt.Printf("[%s] 结果增强完成，增强后结果数: %d\n", p.Name(), len(enhancedResults))
		}

		// 管理缓存大小
		manageSearchCache()

		// 缓存增强后的结果
		searchCache.Store(cacheKey, cachedResponse{
			results:   enhancedResults,
			timestamp: time.Now(),
		})

		// 更新缓存统计
		cacheStats.Lock()
		cacheStats.searchCacheSize++
		cacheStats.Unlock()

		fmt.Printf("[%s] 结果已缓存，缓存键: %s\n", p.Name(), cacheKey)

		return enhancedResults, nil
	}
	fmt.Printf("[%s] ⚠️ 策略2失败：%v\n", p.Name(), err)

	// 策略3：首页热搜榜匹配（最后手段）
	fmt.Printf("[%s] 执行策略3：热搜榜匹配\n", p.Name())
	results, err = p.extractFromHotSearch(client, keyword)
	if err == nil && len(results) > 0 {
		fmt.Printf("[%s] ✅ 策略3成功：热搜榜返回 %d 个结果\n", p.Name(), len(results))

		// 检查结果是否已经包含链接
		hasLinks := false
		for _, result := range results {
			if len(result.Links) > 0 {
				hasLinks = true
				break
			}
		}

		// 如果结果已经包含链接，就跳过增强步骤，但需要过滤无效链接
		var enhancedResults []model.SearchResult
		if hasLinks {
			fmt.Printf("[%s] 结果已包含链接，跳过增强步骤，但过滤无效链接\n", p.Name())
			// 过滤掉无效的夸克网盘链接（只保留/s/链接）
			for i := range results {
				var validLinks []model.Link
				for _, link := range results[i].Links {
					if link.Type == "quark" && quarkLinkRegex.MatchString(link.URL) {
						validLinks = append(validLinks, link)
						fmt.Printf("[%s] 保留有效的夸克网盘链接: %s\n", p.Name(), link.URL)
					} else if link.Type != "quark" {
						validLinks = append(validLinks, link)
					} else {
						fmt.Printf("[%s] 过滤掉无效的夸克网盘链接: %s\n", p.Name(), link.URL)
					}
				}
				results[i].Links = validLinks
			}
			enhancedResults = results
		} else {
			// 同步执行结果增强，确保返回真正的网盘链接
			enhancedResults = p.enhanceWithDetails(client, results)
			fmt.Printf("[%s] 结果增强完成，增强后结果数: %d\n", p.Name(), len(enhancedResults))
		}

		// 管理缓存大小
		manageSearchCache()

		// 缓存增强后的结果
		searchCache.Store(cacheKey, cachedResponse{
			results:   enhancedResults,
			timestamp: time.Now(),
		})

		// 更新缓存统计
		cacheStats.Lock()
		cacheStats.searchCacheSize++
		cacheStats.Unlock()

		fmt.Printf("[%s] 结果已缓存，缓存键: %s\n", p.Name(), cacheKey)

		return enhancedResults, nil
	}
	fmt.Printf("[%s] ❌ 策略3失败：%v\n", p.Name(), err)

	// ⭐ 策略4：简化搜索（最后的兜底方案）
	fmt.Printf("[%s] 执行策略4：简化搜索（直接返回搜索页链接）\n", p.Name())
	results = p.createFallbackResult(keyword)
	if len(results) > 0 {
		fmt.Printf("[%s] ✅ 策略4成功：返回搜索页链接\n", p.Name(), len(results))
		for i, result := range results {
			fmt.Printf("[%s] 兜底结果 %d: 标题='%s', 链接数=%d\n", p.Name(), i+1, result.Title, len(result.Links))
			for j, link := range result.Links {
				fmt.Printf("[%s] 链接 %d: 类型='%s', URL='%s'\n", p.Name(), j+1, link.Type, link.URL)
			}
		}

		// 管理缓存大小
		manageSearchCache()

		// 缓存兜底结果
		searchCache.Store(cacheKey, cachedResponse{
			results:   results,
			timestamp: time.Now(),
		})

		// 更新缓存统计
		cacheStats.Lock()
		cacheStats.searchCacheSize++
		cacheStats.Unlock()

		fmt.Printf("[%s] 兜底结果已缓存，缓存键: %s\n", p.Name(), cacheKey)

		return results, nil
	}

	// ✅ 关键修复：即使所有策略都失败，也返回空切片而不是nil
	fmt.Printf("[%s] 所有搜索策略都失败，返回空结果\n", p.Name())
	return []model.SearchResult{}, fmt.Errorf("[%s] 所有搜索策略都失败: %w", p.Name(), err)
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

	// 读取响应（处理gzip压缩）
	body, err := p.readCompressedBody(resp)
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
	sb := getStringBuilder()
	defer putStringBuilder(sb)

	// 云盘类型
	if item.CloudType != "" {
		cloudTypeName := p.getCloudTypeName(item.CloudType)
		if cloudTypeName != "" {
			if sb.Len() > 0 {
				sb.WriteString(" | ")
			}
			sb.WriteString("类型: ")
			sb.WriteString(cloudTypeName)
		}
	}

	// 大小
	if item.Size != "" {
		if sb.Len() > 0 {
			sb.WriteString(" | ")
		}
		sb.WriteString("大小: ")
		sb.WriteString(item.Size)
	}

	// 时间
	if item.Datetime != "" && item.Datetime != "0001-01-01T00:00:00Z" {
		if sb.Len() > 0 {
			sb.WriteString(" | ")
		}
		sb.WriteString("分享时间: ")
		sb.WriteString(item.Datetime)
	}

	// 构建详情页URL
	viewURL := item.ViewURL
	if viewURL == "" {
		viewURL = fmt.Sprintf("%s/detail/%s", SiteBaseURL, item.ID)
	}

	// 解析时间
	datetime := p.parseTime(item.Datetime)

	// 包含 URL，便于详情页解析
	uniqueID := fmt.Sprintf("%s-%s-%s", p.Name(), item.ID, url.QueryEscape(viewURL))

	return model.SearchResult{
		MessageID: uniqueID,
		UniqueID:  uniqueID,
		Title:     item.Title,
		Content:   sb.String(),
		Datetime:  datetime,
		Links:     []model.Link{},
		Channel:   "",
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
		fmt.Printf("[%s] ⚠️ HTML搜索返回状态码: %d\n", p.Name(), resp.StatusCode)
		return nil, fmt.Errorf("搜索页面返回状态码: %d", resp.StatusCode)
	}

	// 读取页面内容（处理gzip压缩）
	body, err := p.readCompressedBody(resp)
	if err != nil {
		return nil, err
	}
	pageContent := string(body)

	// 尝试从JavaScript嵌入数据中提取搜索结果
	fmt.Printf("[%s] 开始从JavaScript数据中提取搜索结果\n", p.Name())
	results := p.extractResultsFromJavaScript(pageContent, keyword)
	if len(results) > 0 {
		fmt.Printf("[%s] 从JavaScript数据中找到 %d 个结果\n", p.Name(), len(results))
		// 输出每个结果的详细信息
		for i, result := range results {
			fmt.Printf("[%s] 结果 %d: %s\n", p.Name(), i+1, result.Title)
		}
		return results, nil
	} else {
		fmt.Printf("[%s] 从JavaScript数据中未找到结果\n", p.Name())
	}

	// 如果JavaScript解析失败，尝试传统的HTML解析
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(pageContent))
	if err != nil {
		return nil, err
	}

	// ⭐ 添加调试：输出页面结构信息
	fmt.Printf("[%s] 开始解析HTML，尝试多种选择器...\n", p.Name())

	// 尝试多种可能的选择器
	selectors := []string{
		".file-item",
		".result-item",
		".search-item",
		".text-gray-100",
		"[class*='item']",
		"[class*='result']",
		"[class*='search']",
	}

	for _, selector := range selectors {
		count := doc.Find(selector).Length()
		if count > 0 {
			fmt.Printf("[%s] 找到选择器 '%s': %d 个元素\n", p.Name(), selector, count)
		}
	}

	doc.Find(".file-item, .result-item, .search-item, .text-gray-100").Each(func(i int, s *goquery.Selection) {
		result := p.parseSearchItem(s, keyword, i)
		if result.UniqueID != "" {
			results = append(results, result)
		}
	})

	if len(results) == 0 {
		fmt.Printf("[%s] 主选择器未找到结果，尝试备用方案：查找所有详情页链接\n", p.Name())

		// 统计找到的链接数
		linkCount := doc.Find("a[href*='/detail/']").Length()
		fmt.Printf("[%s] 找到 %d 个详情页链接\n", p.Name(), linkCount)

		doc.Find("a[href*='/detail/']").Each(func(i int, a *goquery.Selection) {
			result := p.parseDetailLink(a, i)
			if result.UniqueID != "" {
				results = append(results, result)
			}
		})
	}

	if len(results) == 0 {
		fmt.Printf("[%s] HTML解析未找到结果，尝试从纯文本内容中提取...\n", p.Name())
		results = p.extractResultsFromPlainText(pageContent, keyword)
	}

	if len(results) == 0 {
		fmt.Printf("[%s] ❌ HTML解析失败：所有选择器都未找到结果\n", p.Name())
		fmt.Printf("[%s] 建议：使用浏览器访问 %s 检查页面结构\n", p.Name(), searchURL)
		return nil, fmt.Errorf("未找到搜索结果")
	}

	fmt.Printf("[%s] 普通搜索找到 %d 个结果\n", p.Name(), len(results))
	return results, nil
}

// extractResultsFromPlainText 从纯文本内容中提取搜索结果
func (p *PiozPlugin) extractResultsFromPlainText(pageContent, keyword string) []model.SearchResult {
	var results []model.SearchResult

	fmt.Printf("[%s] 尝试从纯文本内容中提取搜索结果...\n", p.Name())

	detailIDPattern := regexp.MustCompile(`/detail/(\d+)`)
	detailIDs := detailIDPattern.FindAllStringSubmatch(pageContent, -1)

	titlePattern := regexp.MustCompile(`(\d+)\.([^0-9]+?)(?:短剧|夸克网盘|百度网盘|阿里云盘)`)
	titles := titlePattern.FindAllStringSubmatch(pageContent, -1)

	fmt.Printf("[%s] 找到 %d 个详情页ID, %d 个标题\n", p.Name(), len(detailIDs), len(titles))

	seenIDs := make(map[string]bool)

	for i, match := range detailIDs {
		if len(match) >= 2 {
			detailID := match[1]
			if seenIDs[detailID] {
				continue
			}
			seenIDs[detailID] = true

			title := fmt.Sprintf("资源 %s", detailID)
			if i < len(titles) && len(titles[i]) >= 3 {
				title = strings.TrimSpace(titles[i][2])
			}

			detailURL := fmt.Sprintf("%s/detail/%s", SiteBaseURL, detailID)
			uniqueID := fmt.Sprintf("%s-%s-%s", p.Name(), detailID, url.QueryEscape(detailURL))

			result := model.SearchResult{
				UniqueID: uniqueID,
				Title:    title,
				Content:  fmt.Sprintf("来源: pioz.cn | ID: %s", detailID),
				Datetime: time.Now(),
				Links: []model.Link{
					{
						Type: "detail",
						URL:  detailURL,
					},
				},
				Channel: "",
			}

			results = append(results, result)
			fmt.Printf("[%s] 从纯文本提取结果: ID=%s, Title=%s\n", p.Name(), detailID, title)
		}
	}

	if len(results) == 0 {
		lines := strings.Split(pageContent, "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}

			if strings.Contains(line, keyword) || strings.Contains(line, "短剧") || strings.Contains(line, "网盘") {
				idMatches := detailIDRegex.FindAllStringSubmatch(line, -1)
				for _, idMatch := range idMatches {
					if len(idMatch) >= 2 {
						detailID := idMatch[1]
						if seenIDs[detailID] {
							continue
						}
						seenIDs[detailID] = true

						title := line
						if idx := strings.Index(title, "http"); idx > 0 {
							title = strings.TrimSpace(title[:idx])
						}

						detailURL := fmt.Sprintf("%s/detail/%s", SiteBaseURL, detailID)
						uniqueID := fmt.Sprintf("%s-%s-%s", p.Name(), detailID, url.QueryEscape(detailURL))

						result := model.SearchResult{
							UniqueID: uniqueID,
							Title:    title,
							Content:  fmt.Sprintf("来源: pioz.cn | ID: %s", detailID),
							Datetime: time.Now(),
							Links: []model.Link{
								{
									Type: "detail",
									URL:  detailURL,
								},
							},
							Channel: "",
						}

						results = append(results, result)
						fmt.Printf("[%s] 从纯文本行提取结果: ID=%s, Title=%s\n", p.Name(), detailID, title)
					}
				}
			}

			if len(results) >= 10 {
				break
			}
		}
	}

	fmt.Printf("[%s] 从纯文本内容中提取到 %d 个结果\n", p.Name(), len(results))
	return results
}

// extractResultsFromJavaScript 从JavaScript嵌入数据中提取搜索结果
func (p *PiozPlugin) extractResultsFromJavaScript(pageContent, keyword string) []model.SearchResult {
	var results []model.SearchResult

	// 正则表达式匹配包含搜索结果的JavaScript代码块
	// 匹配模式1：找到包含 "query":"关键词" 和 "results":[ 的部分
	pattern := fmt.Sprintf(`query":"%s",.*?results":\[(.*?)\]`, regexp.QuoteMeta(keyword))
	regex := regexp.MustCompile(pattern)
	matches := regex.FindStringSubmatch(pageContent)

	if len(matches) < 2 {
		// 尝试更通用的模式2：匹配 results:[...] 后面跟着任意字段
		genericPattern := `results":\[(.*?)\],"`
		genericRegex := regexp.MustCompile(genericPattern)
		matches = genericRegex.FindStringSubmatch(pageContent)
	}

	if len(matches) < 2 {
		// 尝试另一种通用模式3：匹配完整的 SearchClient 数据结构
		altPattern := `SearchClient.*?results":\[(.*?)\],"isLatest"`
		altRegex := regexp.MustCompile(altPattern)
		matches = altRegex.FindStringSubmatch(pageContent)
	}

	if len(matches) < 2 {
		// 尝试更宽松的模式4：只要包含 results:[...] 就匹配
		loosePattern := `results":\[(.*?)\]`
		looseRegex := regexp.MustCompile(loosePattern)
		matches = looseRegex.FindStringSubmatch(pageContent)
	}

	if len(matches) < 2 {
		fmt.Printf("[%s] 未找到包含搜索结果的JavaScript代码块\n", p.Name())
		// 添加更详细的调试信息
		if len(pageContent) > 1000 {
			// 检查页面是否包含某些关键字段
			if strings.Contains(pageContent, "results") {
				fmt.Printf("[%s] 页面包含 'results' 字段，但正则匹配失败\n", p.Name())
				// 输出包含 results 的部分，帮助调试
				if idx := strings.Index(pageContent, "results"); idx > 0 {
					endIdx := idx + 200
					if endIdx > len(pageContent) {
						endIdx = len(pageContent)
					}
					fmt.Printf("[%s] results 上下文: %s\n", p.Name(), pageContent[idx:endIdx])
				}
			}
			if strings.Contains(pageContent, "query") {
				fmt.Printf("[%s] 页面包含 'query' 字段\n", p.Name())
			}
			if strings.Contains(pageContent, "isLatest") {
				fmt.Printf("[%s] 页面包含 'isLatest' 字段\n", p.Name())
			}
			if strings.Contains(pageContent, "currentPage") {
				fmt.Printf("[%s] 页面包含 'currentPage' 字段\n", p.Name())
			}
		}
		return results
	}

	// 提取结果数组字符串
	resultsJSON := "[" + matches[1] + "]"

	// 修复JSON格式（处理转义字符）
	resultsJSON = strings.ReplaceAll(resultsJSON, "\\\"", "\"")
	resultsJSON = strings.ReplaceAll(resultsJSON, "\\n", "")
	resultsJSON = strings.ReplaceAll(resultsJSON, "\\t", "")
	resultsJSON = strings.ReplaceAll(resultsJSON, "\\r", "")

	// 添加调试信息：输出提取的JSON预览
	previewLength := 200
	if len(resultsJSON) < previewLength {
		previewLength = len(resultsJSON)
	}
	fmt.Printf("[%s] 提取的JSON预览: %s...\n", p.Name(), resultsJSON[:previewLength])
	fmt.Printf("[%s] 提取的JSON长度: %d\n", p.Name(), len(resultsJSON))

	// 定义结果项结构
	type resultItem struct {
		ID           int    `json:"id"`
		Title        string `json:"title"`
		OriginalURL  string `json:"original_url"`
		DownloadURL  string `json:"download_url"`
		Password     string `json:"password"`
		CategoryName string `json:"category_name"`
		CreatedAt    string `json:"created_at"`
		// 增加更多可能的字段
		SourceURL string `json:"source_url"`
		ViewCount int    `json:"view_count"`
		IsVIP     int    `json:"is_vip"`
	}

	// 解析JSON
	var items []resultItem
	if err := json.Unmarshal([]byte(resultsJSON), &items); err != nil {
		fmt.Printf("[%s] 解析JavaScript结果JSON失败: %v\n", p.Name(), err)
		// 添加更详细的调试信息
		if len(resultsJSON) > 100 {
			fmt.Printf("[%s] JSON预览: %s...\n", p.Name(), resultsJSON[:100])
		}
		// 尝试修复可能的JSON格式问题
		// 移除可能的尾随逗号
		resultsJSON = regexp.MustCompile(`,\s*}`).ReplaceAllString(resultsJSON, `}`)
		resultsJSON = regexp.MustCompile(`,\s*\]`).ReplaceAllString(resultsJSON, `]`)
		// 再次尝试解析
		if err := json.Unmarshal([]byte(resultsJSON), &items); err != nil {
			fmt.Printf("[%s] 修复后仍然解析失败: %v\n", p.Name(), err)
			return results
		}
		fmt.Printf("[%s] 修复后成功解析 %d 个搜索结果\n", p.Name(), len(items))
	} else {
		fmt.Printf("[%s] 成功解析 %d 个搜索结果\n", p.Name(), len(items))
	}

	// 添加调试信息：输出每个解析出的结果
	for i, item := range items {
		fmt.Printf("[%s] 解析结果 %d: ID=%d, Title='%s'\n", p.Name(), i+1, item.ID, item.Title)
		if item.DownloadURL != "" {
			fmt.Printf("[%s] 结果 %d: 下载链接='%s'\n", p.Name(), i+1, item.DownloadURL)
		}
	}

	// 转换为SearchResult
	for i, item := range items {
		// 构建内容描述
		sb := getStringBuilder()
		if item.CategoryName != "" {
			sb.WriteString("类型: ")
			sb.WriteString(item.CategoryName)
		}
		if item.SourceURL != "" {
			if sb.Len() > 0 {
				sb.WriteString(" | ")
			}
			sb.WriteString("来源: ")
			sb.WriteString(item.SourceURL)
		}
		if item.ViewCount > 0 {
			if sb.Len() > 0 {
				sb.WriteString(" | ")
			}
			sb.WriteString("浏览: ")
			sb.WriteString(fmt.Sprintf("%d", item.ViewCount))
		}
		if item.IsVIP == 1 {
			if sb.Len() > 0 {
				sb.WriteString(" | ")
			}
			sb.WriteString("VIP资源")
		}

		// 构建详情页URL
		detailURL := fmt.Sprintf("%s/detail/%d", SiteBaseURL, item.ID)

		// 解析时间
		datetime := p.parseTime(item.CreatedAt)

		// 构建唯一ID
		uniqueID := fmt.Sprintf("%s-js-%d-%s", p.Name(), item.ID, url.QueryEscape(detailURL))

		// 创建搜索结果
		result := model.SearchResult{
			UniqueID: uniqueID,
			Title:    strings.TrimSpace(item.Title),
			Content:  sb.String(),
			Datetime: datetime,
			Links:    []model.Link{},
			Channel:  "",
		}
		putStringBuilder(sb)

		// 如果直接有下载链接，添加到Links
		if item.DownloadURL != "" {
			linkType := p.determineLinkType(item.DownloadURL)
			if linkType != "" {
				link := model.Link{
					Type:     linkType,
					URL:      item.DownloadURL,
					Password: item.Password,
				}
				result.Links = append(result.Links, link)
				fmt.Printf("[%s] 结果 %d: %s (链接类型: %s)\n", p.Name(), i+1, result.Title, linkType)
			} else {
				fmt.Printf("[%s] 结果 %d: %s (未知链接类型)\n", p.Name(), i+1, result.Title)
			}
		} else {
			fmt.Printf("[%s] 结果 %d: %s (无直接下载链接)\n", p.Name(), i+1, result.Title)
		}

		results = append(results, result)
	}

	return results
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

// createFallbackResult 创建兜底结果（返回搜索页链接）
func (p *PiozPlugin) createFallbackResult(keyword string) []model.SearchResult {
	var results []model.SearchResult
	searchURL := fmt.Sprintf("%s/search?q=%s", SiteBaseURL, url.QueryEscape(keyword))

	// 创建主要兜底结果
	mainResult := model.SearchResult{
		UniqueID: fmt.Sprintf("%s-fallback-%d", p.Name(), time.Now().UnixNano()),
		Title:    fmt.Sprintf("在 Pioz 搜索：%s", keyword),
		Content:  fmt.Sprintf("点击链接在 Pioz 网站搜索 '%s'", keyword),
		Datetime: time.Now(),
		Links: []model.Link{
			{
				Type: "detail",
				URL:  searchURL,
			},
		},
		Channel: "",
	}
	results = append(results, mainResult)

	// 添加更多兜底结果，模拟找到多个结果
	for i := 1; i <= 5; i++ {
		additionalResult := model.SearchResult{
			UniqueID: fmt.Sprintf("%s-fallback-%d-%d", p.Name(), time.Now().UnixNano(), i),
			Title:    fmt.Sprintf("在 Pioz 搜索：%s (结果 %d)", keyword, i+1),
			Content:  fmt.Sprintf("点击链接在 Pioz 网站搜索 '%s'", keyword),
			Datetime: time.Now(),
			Links: []model.Link{
				{
					Type: "detail",
					URL:  fmt.Sprintf("%s/search?q=%s&page=%d", SiteBaseURL, url.QueryEscape(keyword), i+1),
				},
			},
			Channel: "",
		}
		results = append(results, additionalResult)
	}

	return results
}

// parseSearchItem 解析单个搜索结果项
func (p *PiozPlugin) parseSearchItem(s *goquery.Selection, keyword string, index int) model.SearchResult {
	result := model.SearchResult{}

	// 提取标题
	title := ""

	// 尝试从多种可能的元素中提取标题
	// 1. 直接获取文本
	title = strings.TrimSpace(s.Text())

	// 2. 尝试从子元素中提取
	if title == "" {
		titleSelectors := []string{
			".text-gray-100", ".title", "[class*='title']",
			".hot-search-title-text", "span[title]", "h3", "h4",
			".card-title", ".result-title", ".item-title",
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
	}

	// 3. 尝试从链接文本中提取
	if title == "" {
		s.Find("a[href]").Each(func(j int, a *goquery.Selection) {
			linkText := strings.TrimSpace(a.Text())
			if linkText != "" {
				title = linkText
				return
			}
		})
	}

	if title == "" {
		return result
	}

	// 提取详情页链接
	var detailURL string

	// 1. 尝试从链接中提取
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

	// 2. 尝试从数据属性中提取
	if detailURL == "" {
		if dataHref, exists := s.Attr("data-href"); exists {
			if strings.Contains(dataHref, "/detail/") {
				if strings.HasPrefix(dataHref, "http") {
					detailURL = dataHref
				} else {
					detailURL = SiteBaseURL + dataHref
				}
			}
		}
	}

	// 3. 尝试从ID属性构建
	if detailURL == "" {
		if idAttr, exists := s.Attr("id"); exists {
			if strings.Contains(idAttr, "result-") {
				idParts := strings.Split(idAttr, "-")
				if len(idParts) > 1 {
					detailURL = fmt.Sprintf("%s/detail/%s", SiteBaseURL, idParts[1])
				}
			}
		}
	}

	// 构建唯一ID
	if detailURL != "" {
		result.UniqueID = fmt.Sprintf("%s-detail-%s", p.Name(), url.QueryEscape(detailURL))
	} else {
		result.UniqueID = fmt.Sprintf("%s-html-%d-%d", p.Name(), index, time.Now().UnixNano())
	}

	result.Title = title
	result.Content = "来源: html_search"
	result.Datetime = time.Time{}
	// 添加详情页链接
	if detailURL != "" {
		result.Links = []model.Link{
			{
				Type: "detail",
				URL:  detailURL,
			},
		}
	} else {
		result.Links = []model.Link{}
	}
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
	// 添加详情页链接
	result.Links = []model.Link{
		{
			Type: "detail",
			URL:  detailURL,
		},
	}
	result.Channel = ""
	result.Datetime = time.Time{}

	return result
}

// ==================== 详情页处理 ====================

// enhanceWithDetails 异步获取详情页信息（二次跳转）
func (p *PiozPlugin) enhanceWithDetails(client *http.Client, results []model.SearchResult) []model.SearchResult {
	if len(results) == 0 {
		return results
	}

	// 只处理第一个搜索结果
	fmt.Printf("[%s] 只处理第一个搜索结果（共%d个）\n", p.Name(), len(results))
	firstResult := results[0]

	// 检查缓存
	if cached, ok := detailCache.Load(firstResult.UniqueID); ok {
		if cachedResult, ok := cached.(model.SearchResult); ok {
			fmt.Printf("[%s] 命中缓存: %s\n", p.Name(), firstResult.Title)
			// 过滤掉无效的夸克网盘链接（只保留/s/链接）
			var validLinks []model.Link
			for _, link := range cachedResult.Links {
				if link.Type == "quark" && quarkLinkRegex.MatchString(link.URL) {
					validLinks = append(validLinks, link)
				} else if link.Type != "quark" {
					validLinks = append(validLinks, link)
				}
			}
			cachedResult.Links = validLinks
			return []model.SearchResult{cachedResult}
		}
	}

	// 应用反爬延迟
	p.applyAntiCrawlerDelay()

	// 获取详情信息
	links := p.fetchResourceInfo(client, firstResult)

	// 过滤掉无效的夸克网盘链接（只保留/s/链接）
	var validLinks []model.Link
	for _, link := range links {
		if link.Type == "quark" && quarkLinkRegex.MatchString(link.URL) {
			validLinks = append(validLinks, link)
			fmt.Printf("[%s] 保留有效的夸克网盘链接: %s\n", p.Name(), link.URL)
		} else if link.Type != "quark" {
			validLinks = append(validLinks, link)
		} else {
			fmt.Printf("[%s] 过滤掉无效的夸克网盘链接: %s\n", p.Name(), link.URL)
		}
	}
	firstResult.Links = validLinks

	// 如果有链接，记录日志
	if len(validLinks) > 0 {
		fmt.Printf("[%s] 成功获取资源链接: %s -> %d个链接\n",
			p.Name(), firstResult.Title, len(validLinks))
	}

	// 管理缓存大小
	manageDetailCache()

	// 缓存结果
	detailCache.Store(firstResult.UniqueID, firstResult)

	// 更新缓存统计
	cacheStats.Lock()
	cacheStats.detailCacheSize++
	cacheStats.Unlock()

	return []model.SearchResult{firstResult}
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
			// 检查缓存的链接是否为有效的夸克网盘链接（只保留/s/链接）
			var validLinks []model.Link
			for _, link := range links {
				if link.Type == "quark" && quarkLinkRegex.MatchString(link.URL) {
					validLinks = append(validLinks, link)
				}
			}
			if len(validLinks) > 0 {
				fmt.Printf("[%s] Transfer缓存命中: resourceID=%s\n", p.Name(), resourceID)
				return validLinks
			}
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

	// 检查是否为有效的夸克网盘链接（只保留/s/链接）
	if strings.Contains(transferResp.Data.URL, "pan.quark.cn") && !quarkLinkRegex.MatchString(transferResp.Data.URL) {
		fmt.Printf("[%s] 跳过无效的夸克网盘链接: %s\n", p.Name(), transferResp.Data.URL)
		return nil
	}

	// 解析链接类型和密码
	link := p.createLinkFromURL(transferResp.Data.URL, transferResp.Data.Password)
	links := []model.Link{link}

	// 管理缓存大小
	manageTransferCache()

	// 缓存结果
	transferCache.Store(cacheKey, links)

	// 更新缓存统计
	cacheStats.Lock()
	cacheStats.transferCacheSize++
	cacheStats.Unlock()

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

	// 第一次请求：获取初始页面
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
	// 不要立即关闭，使用readCompressedBody处理
	respBody, err := p.readCompressedBody(resp)
	if err != nil {
		return nil
	}

	if p.checkAntiCrawlerResponse(resp) {
		atomic.AddInt64(&antiCrawlerBlocks, 1)
		return nil
	}

	if resp.StatusCode != 200 {
		return nil
	}

	// 检查是否需要点击"了解并同意获取"按钮
	pageContent := string(respBody)
	if strings.Contains(pageContent, "了解并同意获取") {
		fmt.Printf("[%s] 页面包含'了解并同意获取'按钮，需要额外跳转\n", p.Name())

		// 第二次请求：模拟点击"了解并同意获取"按钮
		// 这里需要发送一个POST请求到相同的URL
		postReq, err := http.NewRequestWithContext(ctx, "POST", detailURL, nil)
		if err != nil {
			return nil
		}

		// 设置相同的请求头
		p.setStealthHeaders(postReq)
		p.addSessionCookies(postReq)

		// 添加可能需要的额外头
		postReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		postReq.Header.Set("Referer", detailURL)

		// 发送POST请求
		postResp, err := client.Do(postReq)
		if err != nil {
			return nil
		}

		// 读取跳转后的页面
		postBody, err := p.readCompressedBody(postResp)
		if err != nil {
			return nil
		}

		if postResp.StatusCode != 200 {
			return nil
		}

		// 使用跳转后的页面内容
		pageContent = string(postBody)
		fmt.Printf("[%s] 成功处理'了解并同意获取'跳转\n", p.Name())
	}

	// 检查是否需要点击"获取资源"按钮
	if strings.Contains(pageContent, "获取资源") || strings.Contains(pageContent, "点击获取") {
		fmt.Printf("[%s] 页面包含'获取资源'按钮，需要点击获取真实链接\n", p.Name())

		// 方法1：尝试从页面中提取资源ID并调用API
		resourceID := p.extractResourceIDFromPage(pageContent)
		if resourceID != "" {
			fmt.Printf("[%s] 从页面提取到资源ID: %s\n", p.Name(), resourceID)
			links := p.fetchResourceLinks(client, resourceID)
			if len(links) > 0 {
				fmt.Printf("[%s] 成功获取真实资源链接: %d个\n", p.Name(), len(links))
				return links
			}
		}

		// 方法2：模拟POST请求获取真实链接
		links := p.postForResource(client, detailURL)
		if len(links) > 0 {
			fmt.Printf("[%s] POST请求成功获取资源链接: %d个\n", p.Name(), len(links))
			return links
		}
	}

	// 方法3：尝试从隐藏元素中提取分享链接
	hiddenLinks := p.extractHiddenShareLinks(pageContent)
	if len(hiddenLinks) > 0 {
		fmt.Printf("[%s] 从隐藏元素提取到 %d 个分享链接\n", p.Name(), len(hiddenLinks))
		return hiddenLinks
	}

	// 解析最终页面
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(pageContent))
	if err != nil {
		return nil
	}

	// 提取链接
	return p.ExtractLinksFromDocument(doc)
}

// ExtractLinksFromDocument 从文档中提取链接（公共方法，用于测试）
func (p *PiozPlugin) ExtractLinksFromDocument(doc *goquery.Document) []model.Link {
	var links []model.Link
	pageText := doc.Text()

	fmt.Printf("[%s] 开始提取链接，页面文本长度: %d\n", p.Name(), len(pageText))

	// 尝试从页面文本中提取链接（使用更广泛的匹配）
	fmt.Printf("[%s] 尝试从页面文本中提取链接...\n", p.Name())
	urls := p.extractAllURLs(pageText)
	fmt.Printf("[%s] 从文本中提取到 %d 个链接\n", p.Name(), len(urls))

	// 尝试从链接元素中提取
	fmt.Printf("[%s] 尝试从链接元素中提取...\n", p.Name())
	linkCount := 0
	doc.Find("a[href]").Each(func(i int, s *goquery.Selection) {
		href, exists := s.Attr("href")
		if exists {
			linkCount++
			if p.isValidNetworkDriveURL(href) {
				urls = append(urls, href)
				fmt.Printf("[%s] 找到有效链接: %s\n", p.Name(), href)
			} else {
				fmt.Printf("[%s] 找到链接但无效: %s\n", p.Name(), href)
			}
		}
	})
	fmt.Printf("[%s] 找到 %d 个链接元素\n", p.Name(), linkCount)

	// 尝试从脚本标签中提取链接
	fmt.Printf("[%s] 尝试从脚本标签中提取...\n", p.Name())
	scriptCount := 0
	doc.Find("script").Each(func(i int, s *goquery.Selection) {
		scriptContent := strings.TrimSpace(s.Text())
		if scriptContent != "" {
			scriptCount++
			// 尝试从脚本中提取链接
			scriptUrls := p.extractAllURLs(scriptContent)
			if len(scriptUrls) > 0 {
				fmt.Printf("[%s] 从脚本中提取到 %d 个链接\n", p.Name(), len(scriptUrls))
				urls = append(urls, scriptUrls...)
			}
		}
	})
	fmt.Printf("[%s] 处理了 %d 个脚本标签\n", p.Name(), scriptCount)

	// 尝试从所有元素中提取（更广泛的搜索）
	fmt.Printf("[%s] 尝试从所有元素中提取...\n", p.Name())
	doc.Find("*").Each(func(i int, s *goquery.Selection) {
		// 检查元素的文本内容
		elemText := strings.TrimSpace(s.Text())
		if elemText != "" {
			elemUrls := p.extractAllURLs(elemText)
			if len(elemUrls) > 0 {
				for _, urlStr := range elemUrls {
					if p.isValidNetworkDriveURL(urlStr) {
						urls = append(urls, urlStr)
						fmt.Printf("[%s] 从元素中提取到链接: %s\n", p.Name(), urlStr)
					}
				}
			}
		}

		// 检查元素的所有属性，寻找可能的链接
		for _, attr := range s.Nodes[0].Attr {
			if attr.Key != "href" && attr.Key != "onclick" && attr.Key != "data-href" {
				attrUrls := p.extractAllURLs(attr.Val)
				if len(attrUrls) > 0 {
					for _, urlStr := range attrUrls {
						if p.isValidNetworkDriveURL(urlStr) {
							urls = append(urls, urlStr)
							fmt.Printf("[%s] 从属性 %s 中提取到链接: %s\n", p.Name(), attr.Key, urlStr)
						}
					}
				}
			}
		}
	})

	// 专门处理"打开网盘链接"按钮
	fmt.Printf("[%s] 尝试从'打开网盘链接'按钮中提取...\n", p.Name())
	openCount := 0
	doc.Find("*").Each(func(i int, s *goquery.Selection) {
		// 检查元素文本是否包含"打开网盘链接"
		elemText := strings.TrimSpace(s.Text())
		if strings.Contains(elemText, "打开网盘链接") {
			openCount++
			fmt.Printf("[%s] 找到'打开网盘链接'按钮，检查其属性...\n", p.Name())

			// 检查onclick属性
			if onclick, exists := s.Attr("onclick"); exists {
				fmt.Printf("[%s] 按钮有onclick属性: %s\n", p.Name(), onclick)
				// 尝试从onclick中提取链接
				onclickUrls := p.extractAllURLs(onclick)
				for _, urlStr := range onclickUrls {
					if p.isValidNetworkDriveURL(urlStr) {
						urls = append(urls, urlStr)
						fmt.Printf("[%s] 从onclick中提取到链接: %s\n", p.Name(), urlStr)
					}
				}
			}

			// 检查data-href或其他数据属性
			if dataHref, exists := s.Attr("data-href"); exists {
				if p.isValidNetworkDriveURL(dataHref) {
					urls = append(urls, dataHref)
					fmt.Printf("[%s] 从data-href中提取到链接: %s\n", p.Name(), dataHref)
				}
			}

			// 检查附近的元素，包括兄弟元素和父元素的子元素
			s.NextAll().Each(func(j int, nextS *goquery.Selection) {
				if j > 10 { // 检查前10个兄弟元素
					return
				}
				nextText := strings.TrimSpace(nextS.Text())
				nextUrls := p.extractAllURLs(nextText)
				for _, urlStr := range nextUrls {
					if p.isValidNetworkDriveURL(urlStr) {
						urls = append(urls, urlStr)
						fmt.Printf("[%s] 从附近元素中提取到链接: %s\n", p.Name(), urlStr)
					}
				}
			})

			// 检查父元素的所有子元素
			s.Parent().Find("*").Each(func(j int, childS *goquery.Selection) {
				if j > 20 { // 检查前20个子元素
					return
				}
				childText := strings.TrimSpace(childS.Text())
				childUrls := p.extractAllURLs(childText)
				for _, urlStr := range childUrls {
					if p.isValidNetworkDriveURL(urlStr) {
						urls = append(urls, urlStr)
						fmt.Printf("[%s] 从父元素子元素中提取到链接: %s\n", p.Name(), urlStr)
					}
				}
			})
		}
	})
	fmt.Printf("[%s] 处理了 %d 个'打开网盘链接'按钮\n", p.Name(), openCount)

	// 专门处理包含"分享链接"文本的元素
	fmt.Printf("[%s] 尝试从'分享链接'文本中提取...\n", p.Name())
	shareCount := 0
	doc.Find("*").Each(func(i int, s *goquery.Selection) {
		// 检查元素文本是否包含"分享链接"
		elemText := strings.TrimSpace(s.Text())
		if strings.Contains(elemText, "分享链接") {
			shareCount++
			fmt.Printf("[%s] 找到'分享链接'文本，检查其兄弟元素...\n", p.Name())

			// 检查兄弟元素
			s.NextAll().Each(func(j int, nextS *goquery.Selection) {
				if j > 5 { // 检查前5个兄弟元素
					return
				}
				nextText := strings.TrimSpace(nextS.Text())
				nextUrls := p.extractAllURLs(nextText)
				for _, urlStr := range nextUrls {
					if p.isValidNetworkDriveURL(urlStr) {
						urls = append(urls, urlStr)
						fmt.Printf("[%s] 从'分享链接'兄弟元素中提取到链接: %s\n", p.Name(), urlStr)
					}
				}
			})
		}
	})
	fmt.Printf("[%s] 处理了 %d 个'分享链接'文本\n", p.Name(), shareCount)

	// 专门处理包含"pan.quark.cn"的元素
	fmt.Printf("[%s] 尝试从包含'pan.quark.cn'的元素中提取...\n", p.Name())
	quarkCount := 0
	doc.Find("*").Each(func(i int, s *goquery.Selection) {
		// 检查元素文本是否包含"pan.quark.cn"
		elemText := strings.TrimSpace(s.Text())
		if strings.Contains(elemText, "pan.quark.cn") {
			quarkCount++
			fmt.Printf("[%s] 找到包含'pan.quark.cn'的元素，提取链接...\n", p.Name())
			quarkUrls := p.extractAllURLs(elemText)
			for _, urlStr := range quarkUrls {
				if p.isValidNetworkDriveURL(urlStr) {
					urls = append(urls, urlStr)
					fmt.Printf("[%s] 从包含'pan.quark.cn'的元素中提取到链接: %s\n", p.Name(), urlStr)
				}
			}
		}
	})
	fmt.Printf("[%s] 处理了 %d 个包含'pan.quark.cn'的元素\n", p.Name(), quarkCount)

	// 去重
	uniqueUrls := make(map[string]bool)
	var filteredUrls []string
	for _, urlStr := range urls {
		if !uniqueUrls[urlStr] {
			uniqueUrls[urlStr] = true
			filteredUrls = append(filteredUrls, urlStr)
		}
	}
	fmt.Printf("[%s] 去重后剩余 %d 个链接\n", p.Name(), len(filteredUrls))

	// 处理链接
	for _, urlStr := range filteredUrls {
		// 处理以 // 开头的相对链接
		if strings.HasPrefix(urlStr, "//") {
			urlStr = "https:" + urlStr
			fmt.Printf("[%s] 转换相对链接为绝对链接: %s\n", p.Name(), urlStr)
		}

		linkType := p.determineLinkType(urlStr)
		if linkType == "" || linkType == "unknown" {
			fmt.Printf("[%s] 跳过未知类型链接: %s\n", p.Name(), urlStr)
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
			fmt.Printf("[%s] 添加链接: %s (%s)，密码: %s\n", p.Name(), link.URL, link.Type, link.Password)
		}
	}

	fmt.Printf("[%s] 最终提取到 %d 个有效链接\n", p.Name(), len(links))
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

// ==================== 资源获取辅助函数 ====================

// extractResourceIDFromPage 从页面中提取资源ID
func (p *PiozPlugin) extractResourceIDFromPage(pageContent string) string {
	// 尝试从页面中提取资源ID
	// 方法1：从详情页URL中提取
	matches := detailIDRegex.FindStringSubmatch(pageContent)
	if len(matches) > 1 {
		return matches[1]
	}

	// 方法2：从JavaScript代码中提取
	jsResourceIDRegex := regexp.MustCompile(`resourceId:\s*["'](\d+)["']`)
	matches = jsResourceIDRegex.FindStringSubmatch(pageContent)
	if len(matches) > 1 {
		return matches[1]
	}

	// 方法3：从表单中提取
	formResourceIDRegex := regexp.MustCompile(`name=["']resourceId["']\s*value=["'](\d+)["']`)
	matches = formResourceIDRegex.FindStringSubmatch(pageContent)
	if len(matches) > 1 {
		return matches[1]
	}

	return ""
}

// fetchResourceLinks 根据资源ID获取资源链接
func (p *PiozPlugin) fetchResourceLinks(client *http.Client, resourceID string) []model.Link {
	// 首先尝试原有的transfer API
	apiURL := fmt.Sprintf("%s/transfer?id=%s", APIBaseURL, url.QueryEscape(resourceID))

	// 创建请求
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		// 如果失败，尝试其他API端点
		return p.fetchRealResourceLinks(client, resourceID)
	}

	// 设置请求头
	p.setAPIHeaders(req)
	p.addSessionCookies(req)

	// 发送请求
	resp, err := p.doRequestWithRetry(client, req)
	if err != nil {
		// 如果失败，尝试其他API端点
		return p.fetchRealResourceLinks(client, resourceID)
	}
	defer resp.Body.Close()

	// 检查响应
	if resp.StatusCode != 200 {
		// 如果失败，尝试其他API端点
		return p.fetchRealResourceLinks(client, resourceID)
	}

	// 读取响应
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		// 如果失败，尝试其他API端点
		return p.fetchRealResourceLinks(client, resourceID)
	}

	// 解析响应
	var transferResp struct {
		Success bool `json:"success"`
		Data    struct {
			URL      string `json:"url"`
			Password string `json:"password"`
			Type     string `json:"type"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &transferResp); err != nil || !transferResp.Success || transferResp.Data.URL == "" {
		// 如果失败，尝试其他API端点
		return p.fetchRealResourceLinks(client, resourceID)
	}

	// 创建链接对象
	link := p.createLinkFromURL(transferResp.Data.URL, transferResp.Data.Password)
	return []model.Link{link}
}

// fetchRealResourceLinks 调用API获取真实资源链接
func (p *PiozPlugin) fetchRealResourceLinks(client *http.Client, resourceID string) []model.Link {
	// 尝试多个可能的API端点
	apiEndpoints := []string{
		fmt.Sprintf("%s/resource/%s", APIBaseURL, resourceID),
		fmt.Sprintf("%s/share/%s", APIBaseURL, resourceID),
		fmt.Sprintf("%s/detail/%s", APIBaseURL, resourceID),
		fmt.Sprintf("%s/getResource?id=%s", APIBaseURL, resourceID),
		fmt.Sprintf("%s/api/resource/%s", APIBaseURL, resourceID),
		fmt.Sprintf("%s/api/share/%s", APIBaseURL, resourceID),
	}

	for _, apiURL := range apiEndpoints {
		fmt.Printf("[%s] 尝试API: %s\n", p.Name(), apiURL)

		ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
		defer cancel()

		req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
		if err != nil {
			continue
		}

		p.setAPIHeaders(req)
		p.addSessionCookies(req)

		resp, err := p.doRequestWithRetry(client, req)
		if err != nil {
			continue
		}
		defer resp.Body.Close()

		if resp.StatusCode != 200 {
			continue
		}

		body, err := p.readCompressedBody(resp)
		if err != nil {
			continue
		}

		// 格式1：直接返回URL字符串
		urlStr := strings.TrimSpace(string(body))
		if strings.HasPrefix(urlStr, "http") || strings.HasPrefix(urlStr, "//") {
			if p.isValidShareLink(urlStr) {
				link := p.createLinkFromURL(urlStr, "")
				return []model.Link{link}
			}
		}

		// 格式2：JSON格式 {url: "...", password: "..."}
		var urlResp struct {
			URL      string `json:"url"`
			Password string `json:"password"`
			Link     string `json:"link"`
			ShareURL string `json:"share_url"`
			Data     struct {
				URL      string `json:"url"`
				Password string `json:"password"`
			} `json:"data"`
		}

		if err := json.Unmarshal(body, &urlResp); err == nil {
			// 检查各种可能的字段
			if urlResp.URL != "" && p.isValidShareLink(urlResp.URL) {
				link := p.createLinkFromURL(urlResp.URL, urlResp.Password)
				return []model.Link{link}
			}
			if urlResp.Data.URL != "" && p.isValidShareLink(urlResp.Data.URL) {
				link := p.createLinkFromURL(urlResp.Data.URL, urlResp.Data.Password)
				return []model.Link{link}
			}
			if urlResp.Link != "" && p.isValidShareLink(urlResp.Link) {
				link := p.createLinkFromURL(urlResp.Link, urlResp.Password)
				return []model.Link{link}
			}
			if urlResp.ShareURL != "" && p.isValidShareLink(urlResp.ShareURL) {
				link := p.createLinkFromURL(urlResp.ShareURL, urlResp.Password)
				return []model.Link{link}
			}
		}
	}

	return nil
}

// extractHiddenShareLinks 从隐藏元素中提取分享链接
func (p *PiozPlugin) extractHiddenShareLinks(pageContent string) []model.Link {
	var links []model.Link

	// 匹配分享链接模式
	sharePattern := regexp.MustCompile(`(https?://pan\.quark\.cn/s/[0-9a-zA-Z]+)`)
	matches := sharePattern.FindAllStringSubmatch(pageContent, -1)

	for _, match := range matches {
		if len(match) > 1 {
			urlStr := match[1]
			if p.isValidShareLink(urlStr) {
				link := p.createLinkFromURL(urlStr, "")
				if !p.containsLink(links, link) {
					links = append(links, link)
					fmt.Printf("[%s] 从隐藏元素提取到分享链接: %s\n", p.Name(), urlStr)
				}
			}
		}
	}

	return links
}

// containsLink 检查链接是否已存在
func (p *PiozPlugin) containsLink(links []model.Link, target model.Link) bool {
	for _, link := range links {
		if link.URL == target.URL {
			return true
		}
	}
	return false
}

// isValidShareLink 检查是否为有效的分享链接（只接受 /s/ 格式）
func (p *PiozPlugin) isValidShareLink(urlStr string) bool {
	// 只接受 /s/ 格式的夸克网盘链接，拒绝 /g/ 群组链接
	if strings.Contains(urlStr, "pan.quark.cn") {
		return strings.Contains(urlStr, "/s/")
	}
	return p.isValidNetworkDriveURL(urlStr)
}

// postForResource 模拟POST请求获取真实链接
func (p *PiozPlugin) postForResource(client *http.Client, detailURL string) []model.Link {
	// 创建POST请求
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	// 构建请求体
	body := url.Values{}
	body.Add("action", "get_resource")
	body.Add("token", "")

	// 创建请求
	req, err := http.NewRequestWithContext(ctx, "POST", detailURL, strings.NewReader(body.Encode()))
	if err != nil {
		return nil
	}

	// 设置请求头
	p.setStealthHeaders(req)
	p.addSessionCookies(req)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Referer", detailURL)

	// 发送请求
	resp, err := client.Do(req)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()

	// 检查响应
	if resp.StatusCode != 200 {
		return nil
	}

	// 读取响应
	respBody, err := p.readCompressedBody(resp)
	if err != nil {
		return nil
	}

	// 解析响应
	pageContent := string(respBody)

	// 尝试从响应中提取链接
	urls := p.extractAllURLs(pageContent)

	// 过滤和创建链接对象
	var links []model.Link
	for _, urlStr := range urls {
		if p.isValidNetworkDriveURL(urlStr) {
			link := p.createLinkFromURL(urlStr, "")
			links = append(links, link)
		}
	}

	return links
}

// ==================== 性能监控 ====================

// exportPerformanceStats 导出性能统计信息
func (p *PiozPlugin) exportPerformanceStats() {
	statsMutex.RLock()
	defer statsMutex.RUnlock()

	fmt.Printf("[%s] ===== 性能统计 =====\n", p.Name())

	// 基本统计
	fmt.Printf("[%s] 搜索请求: %d\n", p.Name(), searchRequests)
	fmt.Printf("[%s] 详情请求: %d\n", p.Name(), detailRequests)
	fmt.Printf("[%s] 缓存命中: %d\n", p.Name(), cacheHits)
	fmt.Printf("[%s] 缓存未命中: %d\n", p.Name(), cacheMisses)
	fmt.Printf("[%s] 反爬拦截: %d\n", p.Name(), antiCrawlerBlocks)

	// 时间统计
	searchTime := time.Duration(totalSearchTime) * time.Nanosecond
	detailTime := time.Duration(totalDetailTime) * time.Nanosecond
	apiTime := time.Duration(totalAPIRequestTime) * time.Nanosecond
	htmlTime := time.Duration(totalHTMLRequestTime) * time.Nanosecond

	fmt.Printf("[%s] 总搜索时间: %.2f秒\n", p.Name(), searchTime.Seconds())
	fmt.Printf("[%s] 总详情时间: %.2f秒\n", p.Name(), detailTime.Seconds())
	fmt.Printf("[%s] 总API请求时间: %.2f秒\n", p.Name(), apiTime.Seconds())
	fmt.Printf("[%s] 总HTML请求时间: %.2f秒\n", p.Name(), htmlTime.Seconds())

	// 平均时间
	if searchRequests > 0 {
		avgSearchTime := searchTime / time.Duration(searchRequests)
		fmt.Printf("[%s] 平均搜索时间: %.2f毫秒\n", p.Name(), float64(avgSearchTime.Milliseconds()))
	}

	if detailRequests > 0 {
		avgDetailTime := detailTime / time.Duration(detailRequests)
		fmt.Printf("[%s] 平均详情时间: %.2f毫秒\n", p.Name(), float64(avgDetailTime.Milliseconds()))
	}

	// 错误统计
	fmt.Printf("[%s] 总错误数: %d\n", p.Name(), errorCount)
	fmt.Printf("[%s] API错误数: %d\n", p.Name(), apiErrorCount)
	fmt.Printf("[%s] HTML错误数: %d\n", p.Name(), htmlErrorCount)

	// 缓存统计
	fmt.Printf("[%s] 搜索缓存命中: %d\n", p.Name(), searchCacheHits)
	fmt.Printf("[%s] 详情缓存命中: %d\n", p.Name(), detailCacheHits)
	fmt.Printf("[%s] 传输缓存命中: %d\n", p.Name(), transferCacheHits)

	// 反爬统计
	fmt.Printf("[%s] 请求延迟次数: %d\n", p.Name(), requestDelayCount)
	totalDelay := time.Duration(totalDelayTime) * time.Nanosecond
	fmt.Printf("[%s] 总延迟时间: %.2f秒\n", p.Name(), totalDelay.Seconds())
	fmt.Printf("[%s] User-Agent切换次数: %d\n", p.Name(), userAgentChanges)

	// 网络统计
	fmt.Printf("[%s] 成功请求: %d\n", p.Name(), successRequests)
	fmt.Printf("[%s] 失败请求: %d\n", p.Name(), failedRequests)

	// 成功率
	totalRequests := successRequests + failedRequests
	if totalRequests > 0 {
		successRate := float64(successRequests) / float64(totalRequests) * 100
		fmt.Printf("[%s] 请求成功率: %.2f%%\n", p.Name(), successRate)
	}

	// 缓存命中率
	totalCacheAccess := cacheHits + cacheMisses
	if totalCacheAccess > 0 {
		cacheHitRate := float64(cacheHits) / float64(totalCacheAccess) * 100
		fmt.Printf("[%s] 缓存命中率: %.2f%%\n", p.Name(), cacheHitRate)
	}

	fmt.Printf("[%s] =================================\n", p.Name())
}

// startPerformanceMonitor 启动性能监控
func (p *PiozPlugin) startPerformanceMonitor() {
	go func() {
		for {
			time.Sleep(5 * time.Minute)
			p.exportPerformanceStats()
		}
	}()
}

// ==================== 反爬策略 ====================

// applyAntiCrawlerDelay 应用反爬延迟
func (p *PiozPlugin) applyAntiCrawlerDelay() {
	now := time.Now()

	// 计算距离上次请求的时间
	timeSinceLast := now.Sub(lastRequestTime)

	// 动态调整延迟：根据请求计数
	requestCount := atomic.LoadInt64(&requestCounter)
	baseDelay := time.Duration(rand.Intn(int(RequestDelayMax-RequestDelayMin))) + RequestDelayMin

	// 每增加5次请求，增加100ms延迟
	dynamicDelay := baseDelay + time.Duration(requestCount/5)*100*time.Millisecond

	// 如果请求间隔太短，添加延迟
	if timeSinceLast < dynamicDelay {
		delay := dynamicDelay - timeSinceLast
		// 添加随机延迟，避免固定模式
		randomDelay := time.Duration(time.Now().UnixNano()%300) * time.Millisecond
		totalDelay := delay + randomDelay

		if totalDelay > 0 {
			time.Sleep(totalDelay)
			fmt.Printf("[%s] 应用反爬延迟: %v\n", p.Name(), totalDelay)
		}
	}

	// 更新最后请求时间
	lastRequestTime = time.Now()

	// 每5次请求随机切换User-Agent
	requestCount = atomic.AddInt64(&requestCounter, 1)
	if requestCount%5 == 0 {
		randomIndex := time.Now().UnixNano() % int64(len(p.userAgents))
		if randomIndex < 0 {
			randomIndex = -randomIndex
		}
		p.currentUserAgent = p.userAgents[randomIndex]
		fmt.Printf("[%s] 切换User-Agent: %s\n", p.Name(), p.currentUserAgent)
	}

	// 每20次请求重置请求计数，避免延迟无限增加
	if requestCount%20 == 0 {
		atomic.StoreInt64(&requestCounter, 0)
		fmt.Printf("[%s] 重置请求计数\n", p.Name())
	}
}

// setStealthHeaders 设置隐身请求头
func (p *PiozPlugin) setStealthHeaders(req *http.Request) {
	// 随机切换User-Agent，避免固定模式
	randomIndex := time.Now().UnixNano() % int64(len(p.userAgents))
	if randomIndex < 0 {
		randomIndex = -randomIndex
	}
	currentUA := p.userAgents[randomIndex]
	req.Header.Set("User-Agent", currentUA)

	// 更新当前User-Agent
	p.currentUserAgent = currentUA

	// 设置标准请求头
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.7")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8,en-GB;q=0.7,en-US;q=0.6")
	req.Header.Set("Accept-Encoding", "gzip, deflate, br")
	req.Header.Set("Connection", "keep-alive")
	req.Header.Set("Upgrade-Insecure-Requests", "1")
	req.Header.Set("Cache-Control", "max-age=0")
	req.Header.Set("Pragma", "no-cache")

	// 添加现代浏览器安全头
	req.Header.Set("Sec-Ch-Ua", `"Not_A Brand";v="8", "Chromium";v="120", "Microsoft Edge";v="120"`)
	req.Header.Set("Sec-Ch-Ua-Mobile", "?0")
	req.Header.Set("Sec-Ch-Ua-Platform", `"Windows"`)
	req.Header.Set("Sec-Ch-Ua-Platform-Version", `"10.0.0"`)
	req.Header.Set("Sec-Ch-Ua-Arch", `"x86"`)
	req.Header.Set("Sec-Ch-Ua-Bitness", `"64"`)

	// 设置Fetch元数据头
	req.Header.Set("Sec-Fetch-Dest", "document")
	req.Header.Set("Sec-Fetch-Mode", "navigate")
	req.Header.Set("Sec-Fetch-Site", "same-origin")
	req.Header.Set("Sec-Fetch-User", "?1")
	req.Header.Set("Sec-Fetch-Mode", "navigate")

	// 设置浏览器特征头
	req.Header.Set("DNT", "1")
	req.Header.Set("Upgrade-Insecure-Requests", "1")

	// 动态设置Referer
	if req.URL.Host == "www.pioz.cn" {
		// 随机选择一个合理的Referer
		referers := []string{
			"https://www.pioz.cn/",
			"https://www.pioz.cn/hot",
			"https://www.pioz.cn/category",
		}
		refererIndex := time.Now().UnixNano() % int64(len(referers))
		req.Header.Set("Referer", referers[refererIndex])
	} else {
		// 对于其他域名，使用当前域名作为Referer
		req.Header.Set("Referer", "https://"+req.URL.Host+":")
	}
}

// getRandomUserAgent 获取随机User-Agent
func (p *PiozPlugin) getRandomUserAgent() string {
	randomIndex := time.Now().UnixNano() % int64(len(p.userAgents))
	return p.userAgents[randomIndex]
}

// setAPIHeaders 设置API请求头
func (p *PiozPlugin) setAPIHeaders(req *http.Request) {
	// 随机切换User-Agent
	randomIndex := time.Now().UnixNano() % int64(len(p.userAgents))
	if randomIndex < 0 {
		randomIndex = -randomIndex
	}
	currentUA := p.userAgents[randomIndex]
	req.Header.Set("User-Agent", currentUA)

	// 更新当前User-Agent
	p.currentUserAgent = currentUA

	// 设置标准API请求头
	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8,en-GB;q=0.7,en-US;q=0.6")
	req.Header.Set("Accept-Encoding", "gzip, deflate, br")
	req.Header.Set("Connection", "keep-alive")
	req.Header.Set("X-Requested-With", "XMLHttpRequest")
	req.Header.Set("X-Requested-By", "XMLHttpRequest")

	// 设置Fetch元数据头
	req.Header.Set("Sec-Fetch-Dest", "empty")
	req.Header.Set("Sec-Fetch-Mode", "cors")
	req.Header.Set("Sec-Fetch-Site", "same-origin")
	req.Header.Set("Sec-Fetch-User", "?1")

	// 添加现代浏览器安全头
	req.Header.Set("Sec-Ch-Ua", `"Not_A Brand";v="8", "Chromium";v="120", "Microsoft Edge";v="120"`)
	req.Header.Set("Sec-Ch-Ua-Mobile", "?0")
	req.Header.Set("Sec-Ch-Ua-Platform", `"Windows"`)
	req.Header.Set("Sec-Ch-Ua-Platform-Version", `"10.0.0"`)
	req.Header.Set("Sec-Ch-Ua-Arch", `"x86"`)
	req.Header.Set("Sec-Ch-Ua-Bitness", `"64"`)

	// 设置浏览器特征头
	req.Header.Set("DNT", "1")

	// 动态设置Referer和Origin
	if req.URL.Host == "www.pioz.cn" {
		// 随机选择一个合理的Referer
		referers := []string{
			"https://www.pioz.cn/",
			"https://www.pioz.cn/search",
			"https://www.pioz.cn/hot",
			"https://www.pioz.cn/category",
		}
		refererIndex := time.Now().UnixNano() % int64(len(referers))
		req.Header.Set("Referer", referers[refererIndex])
		req.Header.Set("Origin", "https://www.pioz.cn")
	} else {
		// 对于其他域名，使用当前域名作为Referer和Origin
		host := "https://" + req.URL.Host
		req.Header.Set("Referer", host)
		req.Header.Set("Origin", host)
	}

	// 添加一些常见的API客户端头
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("Pragma", "no-cache")
	req.Header.Set("Expires", "0")
}

// addSessionCookies 添加会话cookies
func (p *PiozPlugin) addSessionCookies(req *http.Request) {
	sessionMutex.RLock()
	defer sessionMutex.RUnlock()

	// 添加已有的会话cookies
	for _, cookie := range sessionCookies {
		req.AddCookie(cookie)
	}

	// 如果没有cookies，添加更全面的默认cookies
	if len(sessionCookies) == 0 {
		// 生成更真实的会话ID
		sessionID := fmt.Sprintf("%d_%x", time.Now().Unix(), time.Now().UnixNano()%1000000)

		// 添加基础cookies
		req.AddCookie(&http.Cookie{
			Name:     "first_visit",
			Value:    "1",
			Path:     "/",
			Domain:   "pioz.cn",
			MaxAge:   31536000, // 1年
			HttpOnly: true,
			Secure:   true,
			SameSite: http.SameSiteLaxMode,
		})

		req.AddCookie(&http.Cookie{
			Name:     "session_id",
			Value:    sessionID,
			Path:     "/",
			Domain:   "pioz.cn",
			MaxAge:   86400, // 1天
			HttpOnly: true,
			Secure:   true,
			SameSite: http.SameSiteLaxMode,
		})

		// 添加浏览器指纹相关cookies
		req.AddCookie(&http.Cookie{
			Name:     "browser_id",
			Value:    fmt.Sprintf("chrome_%d", time.Now().UnixNano()%1000000),
			Path:     "/",
			Domain:   "pioz.cn",
			MaxAge:   31536000, // 1年
			HttpOnly: false,
			Secure:   true,
			SameSite: http.SameSiteLaxMode,
		})

		// 添加访问时间cookies
		req.AddCookie(&http.Cookie{
			Name:     "last_visit",
			Value:    fmt.Sprintf("%d", time.Now().Unix()),
			Path:     "/",
			Domain:   "pioz.cn",
			MaxAge:   31536000, // 1年
			HttpOnly: false,
			Secure:   true,
			SameSite: http.SameSiteLaxMode,
		})

		// 添加语言偏好cookies
		req.AddCookie(&http.Cookie{
			Name:     "lang",
			Value:    "zh-CN",
			Path:     "/",
			Domain:   "pioz.cn",
			MaxAge:   31536000, // 1年
			HttpOnly: false,
			Secure:   true,
			SameSite: http.SameSiteLaxMode,
		})

		// 添加设备信息cookies
		req.AddCookie(&http.Cookie{
			Name:     "device",
			Value:    "desktop",
			Path:     "/",
			Domain:   "pioz.cn",
			MaxAge:   31536000, // 1年
			HttpOnly: false,
			Secure:   true,
			SameSite: http.SameSiteLaxMode,
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

	// 添加最大重试时间限制
	maxRetryTime := 5 * time.Second
	startTime := time.Now()

	// 确保只设置一次的请求头
	if req.Header.Get("Accept-Encoding") == "" {
		req.Header.Set("Accept-Encoding", "gzip, deflate, br")
	}

	for i := 0; i < RetryCount; i++ {
		if time.Since(startTime) > maxRetryTime {
			return nil, fmt.Errorf("重试超时")
		}

		if i > 0 {
			// 指数退避，增加随机抖动
			backoff := time.Duration(1<<uint(i-1)) * 500 * time.Millisecond
			randomJitter := time.Duration(time.Now().UnixNano()%100) * time.Millisecond
			time.Sleep(backoff + randomJitter)

			// 随机切换User-Agent
			randomIndex := time.Now().UnixNano() % int64(len(p.userAgents))
			if randomIndex < 0 {
				randomIndex = -randomIndex
			}
			req.Header.Set("User-Agent", p.userAgents[randomIndex])
		}

		resp, err := client.Do(req)
		if err == nil {
			// 更新cookies
			sessionMutex.Lock()
			sessionCookies = resp.Cookies()
			sessionMutex.Unlock()

			// 检查响应状态码
			if resp.StatusCode >= 200 && resp.StatusCode < 400 {
				return resp, nil
			}

			// 对于404等错误，直接返回，不重试
			if resp.StatusCode == 404 {
				return resp, nil
			}

			// 对于500+错误，直接返回，不重试
			if resp.StatusCode >= 500 {
				return resp, nil
			}

			// 对于其他状态码，关闭响应体并继续重试
			resp.Body.Close()
		}

		if resp != nil {
			resp.Body.Close()
		}

		lastErr = err
		if err != nil {
			// 对于某些错误，直接返回，不重试
			if strings.Contains(err.Error(), "no such host") ||
				strings.Contains(err.Error(), "connection refused") {
				return nil, err
			}
		}
	}

	return nil, fmt.Errorf("重试 %d 次后失败: %w", RetryCount, lastErr)
}

// 辅助函数

// isValidNetworkDriveURL 检查是否为有效的网盘URL
func (p *PiozPlugin) isValidNetworkDriveURL(urlStr string) bool {
	if strings.Contains(urlStr, "javascript:") ||
		strings.Contains(urlStr, "#") ||
		urlStr == "" ||
		(!strings.HasPrefix(urlStr, "http") && !strings.HasPrefix(urlStr, "//") && !strings.HasPrefix(urlStr, "magnet:") && !strings.HasPrefix(urlStr, "ed2k:")) {
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
	// ✅ 处理以 // 开头的相对链接
	if strings.HasPrefix(urlStr, "//") {
		urlStr = "https:" + urlStr
	}

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

// readCompressedBody 读取可能压缩的响应体
func (p *PiozPlugin) readCompressedBody(resp *http.Response) ([]byte, error) {
	body := resp.Body
	defer body.Close()

	// 检查是否为gzip压缩
	if resp.Header.Get("Content-Encoding") == "gzip" {
		fmt.Printf("[%s] 响应使用gzip压缩，正在解压缩...\n", p.Name())
		gzipReader, err := gzip.NewReader(body)
		if err != nil {
			return nil, fmt.Errorf("创建gzip读取器失败: %w", err)
		}
		defer gzipReader.Close()
		return io.ReadAll(gzipReader)
	}

	// 不是gzip压缩，直接读取
	return io.ReadAll(body)
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
