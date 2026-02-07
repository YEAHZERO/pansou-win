package pioz2

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"pansou/model"
	"pansou/plugin"
	"strings"
	"sync"
	"time"
)

const (
	// Pioz 网站基础URL
	SiteBaseURL = "https://www.pioz.cn"
	
	// Pioz API 基础URL
	APIBaseURL = "https://www.pioz.cn/api"
	
	// 搜索页面URL模板
	SearchPageURL = "https://www.pioz.cn/search?q=%s"
	
	// 深度搜索API URL模板
	DeepSearchAPIURL = "https://www.pioz.cn/api/deep-search?kw=%s"
	
	// 超时配置
	DefaultTimeout = 15 * time.Second
	
	// 缓存配置
	CacheTTL = 30 * time.Minute
)

// 缓存相关
var (
	searchCache sync.Map
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

// Pioz2Plugin Pioz2 插件结构
type Pioz2Plugin struct {
	*plugin.BaseAsyncPlugin
	client *http.Client
}

// init 注册插件
func init() {
	plugin.RegisterGlobalPlugin(NewPioz2Plugin())
}

// NewPioz2Plugin 创建新的 Pioz2 插件实例
func NewPioz2Plugin() *Pioz2Plugin {
	// 创建优化的 HTTP 客户端
	client := &http.Client{
		Timeout: DefaultTimeout,
		Transport: &http.Transport{
			MaxIdleConns:        50,
			MaxIdleConnsPerHost: 10,
			IdleConnTimeout:     60 * time.Second,
		},
	}
	
	return &Pioz2Plugin{
		BaseAsyncPlugin: plugin.NewBaseAsyncPlugin("pioz2", 1), // 优先级1 = 高质量数据源
		client:          client,
	}
}

// Search 同步搜索接口
func (p *Pioz2Plugin) Search(keyword string, ext map[string]interface{}) ([]model.SearchResult, error) {
	result, err := p.SearchWithResult(keyword, ext)
	if err != nil {
		return nil, err
	}
	return result.Results, nil
}

// SearchWithResult 带结果统计的搜索接口
func (p *Pioz2Plugin) SearchWithResult(keyword string, ext map[string]interface{}) (model.PluginSearchResult, error) {
	return p.AsyncSearchWithResult(keyword, p.searchImpl, p.MainCacheKey, ext)
}

// searchImpl 实现搜索逻辑
func (p *Pioz2Plugin) searchImpl(client *http.Client, keyword string, ext map[string]interface{}) ([]model.SearchResult, error) {
	fmt.Printf("[%s] 开始搜索，keyword='%s'\n", p.Name(), keyword)
	
	// 检查缓存
	cacheKey := fmt.Sprintf("%s:%s", p.Name(), keyword)
	if cached, ok := searchCache.Load(cacheKey); ok {
		if cachedResp, ok := cached.(cachedResponse); ok {
			if time.Since(cachedResp.timestamp) < CacheTTL {
				fmt.Printf("[%s] 命中缓存，结果数: %d\n", p.Name(), len(cachedResp.results))
				return cachedResp.results, nil
			}
		}
	}
	
	// 使用插件自己的客户端
	if p.client != nil {
		client = p.client
	}
	
	// 调用深度搜索 API
	results, err := p.performDeepSearch(client, keyword)
	if err != nil {
		fmt.Printf("[%s] 深度搜索失败: %v\n", p.Name(), err)
		return nil, err
	}
	
	// 缓存结果
	searchCache.Store(cacheKey, cachedResponse{
		results:   results,
		timestamp: time.Now(),
	})
	
	fmt.Printf("[%s] 搜索完成，结果数: %d\n", p.Name(), len(results))
	return results, nil
}

// performDeepSearch 执行深度搜索API
func (p *Pioz2Plugin) performDeepSearch(client *http.Client, keyword string) ([]model.SearchResult, error) {
	// 构建API URL
	apiURL := fmt.Sprintf(DeepSearchAPIURL, url.QueryEscape(keyword))
	fmt.Printf("[%s] 调用API: %s\n", p.Name(), apiURL)
	
	// 创建请求
	ctx, cancel := context.WithTimeout(context.Background(), DefaultTimeout)
	defer cancel()
	
	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}
	
	// 设置请求头
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")
	req.Header.Set("Referer", SiteBaseURL)
	
	// 发送请求
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求失败: %w", err)
	}
	defer resp.Body.Close()
	
	fmt.Printf("[%s] API响应状态码: %d\n", p.Name(), resp.StatusCode)
	
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("API返回状态码: %d", resp.StatusCode)
	}
	
	// 读取响应
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %w", err)
	}
	
	fmt.Printf("[%s] 响应长度: %d 字节\n", p.Name(), len(body))
	
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
	
	return results, nil
}

// convertToSearchResult 将API结果转换为SearchResult
func (p *Pioz2Plugin) convertToSearchResult(item struct {
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
	contentParts = append(contentParts, "详情: "+viewURL)
	
	// 解析时间
	datetime := p.parseTime(item.Datetime)
	
	// 构建唯一ID
	uniqueID := fmt.Sprintf("%s-%s", p.Name(), item.ID)
	
	// 构建详情页链接
	detailLink := model.Link{
		URL:      viewURL,
		Type:     "detail", // 详情页链接
		Password: "",
	}
	
	return model.SearchResult{
		MessageID: uniqueID,
		UniqueID:  uniqueID,
		Title:     item.Title,
		Content:   fmt.Sprintf("%s", strings.Join(contentParts, " | ")),
		Datetime:  datetime,
		Links:     []model.Link{detailLink}, // 添加详情页链接
		Channel:   "",                       // ⭐ 重要：插件搜索结果Channel必须为空
	}
}

// getCloudTypeName 获取云盘类型名称
func (p *Pioz2Plugin) getCloudTypeName(cloudType string) string {
	typeMap := map[string]string{
		"quark":  "夸克网盘",
		"baidu":  "百度网盘",
		"xunlei": "迅雷云盘",
		"aliyun": "阿里云盘",
		"uc":     "UC网盘",
	}
	
	if name, ok := typeMap[cloudType]; ok {
		return name
	}
	return cloudType
}

// parseTime 解析时间字符串
func (p *Pioz2Plugin) parseTime(timeStr string) time.Time {
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
