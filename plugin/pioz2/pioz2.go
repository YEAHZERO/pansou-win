package pioz2

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"pansou/model"
	"pansou/plugin"
	"strings"
	"time"
)

const (
	DefaultTimeout = 15 * time.Second
	DetailTimeout  = 12 * time.Second
	MaxConcurrency = 3
	CacheTTL       = 30 * time.Minute
	MaxResults     = 30
)

type Pioz2AsyncPlugin struct {
	*plugin.BaseAsyncPlugin
}

func NewPioz2Plugin() *Pioz2AsyncPlugin {
	return &Pioz2AsyncPlugin{
		BaseAsyncPlugin: plugin.NewBaseAsyncPluginWithFilter("pioz2", 1, true),
	}
}

func init() {
	plugin.RegisterGlobalPlugin(NewPioz2Plugin())
}

func (p *Pioz2AsyncPlugin) Search(keyword string, ext map[string]interface{}) ([]model.SearchResult, error) {
	result, err := p.SearchWithResult(keyword, ext)
	if err != nil {
		return nil, err
	}
	return result.Results, nil
}

func (p *Pioz2AsyncPlugin) SearchWithResult(keyword string, ext map[string]interface{}) (model.PluginSearchResult, error) {
	return p.AsyncSearchWithResult(keyword, p.searchImpl, p.MainCacheKey, ext)
}

type DeepSearchResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Total   int    `json:"total"`
	Results []struct {
		ID        string `json:"id"`
		Title     string `json:"title"`
		CloudType string `json:"cloud_type"`
		Datetime  string `json:"datetime"`
	} `json:"results"`
}

func (p *Pioz2AsyncPlugin) searchImpl(client *http.Client, keyword string, ext map[string]interface{}) ([]model.SearchResult, error) {
	fmt.Printf("[pioz2] 开始深度搜索，keyword='%s'\n", keyword)

	apiURL := fmt.Sprintf("https://www.pioz.cn/api/deep-search?kw=%s", url.QueryEscape(keyword))

	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %v", err)
	}

	req.Header.Set("User-Agent", getRandomUA())

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求失败: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("API响应状态码: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %v", err)
	}

	var searchResp DeepSearchResponse
	if err := json.Unmarshal(body, &searchResp); err != nil {
		return nil, fmt.Errorf("解析JSON失败: %v", err)
	}

	if searchResp.Code != 0 {
		return nil, fmt.Errorf("API返回错误: %s", searchResp.Message)
	}

	fmt.Printf("[pioz2] 深度搜索找到 %d 个结果，实际返回 %d 条\n", searchResp.Total, len(searchResp.Results))

	if len(searchResp.Results) == 0 {
		return []model.SearchResult{}, nil
	}

	results := make([]model.SearchResult, 0, len(searchResp.Results))
	jsonResults := make([]map[string]interface{}, 0, len(searchResp.Results))

	maxResults := MaxResults
	if len(searchResp.Results) < maxResults {
		maxResults = len(searchResp.Results)
	}

	for i := 0; i < maxResults; i++ {
		item := searchResp.Results[i]
		linkType := p.mapCloudType(item.CloudType)

		detailURL := fmt.Sprintf("https://www.pioz.cn/detail/%s", item.ID)

		results = append(results, model.SearchResult{
			MessageID: fmt.Sprintf("pioz2-%s", item.ID),
			UniqueID:  fmt.Sprintf("pioz2-%s", item.ID),
			Title:     item.Title,
			Content:   fmt.Sprintf("来源: %s | 时间: %s | 详情页: %s", item.CloudType, item.Datetime, detailURL),
			Links: []model.Link{
				{
					Type:     linkType,
					URL:      detailURL,
					Password: "",
				},
			},
		})

		jsonResults = append(jsonResults, map[string]interface{}{
			"index":      i + 1,
			"id":         item.ID,
			"title":      item.Title,
			"cloud_type": item.CloudType,
			"datetime":   item.Datetime,
			"detail_url": detailURL,
			"link_type":  linkType,
		})

		fmt.Printf("[pioz2] 第 %d 个资源: %s | 来源: %s | 详情页: %s\n", i+1, item.Title, item.CloudType, detailURL)

		if i < maxResults-1 {
			delay := time.Duration(time.Now().UnixNano()%1000+1000) * time.Millisecond
			fmt.Printf("[pioz2] 等待 %v 后继续获取下一个资源...\n", delay)
			time.Sleep(delay)
		}

		if (i+1)%3 == 0 {
			fmt.Printf("[pioz2] 更换User-Agent\n")
		}
	}

	p.saveResultsToFile(keyword, jsonResults)

	fmt.Printf("[pioz2] 插件返回 %d 个有效结果\n", len(results))
	return results, nil
}

func (p *Pioz2AsyncPlugin) mapCloudType(cloudType string) string {
	switch strings.ToLower(cloudType) {
	case "baidu":
		return "baidu"
	case "quark":
		return "quark"
	case "aliyun", "aliyundrive", "alipan":
		return "aliyun"
	case "uc":
		return "uc"
	case "xunlei":
		return "xunlei"
	case "tianyi":
		return "tianyi"
	case "115":
		return "115"
	case "lanzou":
		return "lanzou"
	default:
		return cloudType
	}
}

var userAgents = []string{
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/119.0.0.0 Safari/537.36",
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/118.0.0.0 Safari/537.36",
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/117.0.0.0 Safari/537.36",
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/116.0.0.0 Safari/537.36",
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/115.0.0.0 Safari/537.36",
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/114.0.0.0 Safari/537.36",
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/113.0.0.0 Safari/537.36",
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/112.0.0.0 Safari/537.36",
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.0 Safari/605.1.15",
}

var uaIndex = 0

func getRandomUA() string {
	ua := userAgents[uaIndex]
	uaIndex = (uaIndex + 1) % len(userAgents)
	return ua
}

func (p *Pioz2AsyncPlugin) saveResultsToFile(keyword string, results []map[string]interface{}) {
	timestamp := time.Now().Format("20060102_150405")
	filename := fmt.Sprintf("pioz2_search_%s_%s.json", timestamp, keyword)

	output := map[string]interface{}{
		"keyword":   keyword,
		"timestamp": time.Now().Format("2006-01-02 15:04:05"),
		"total":     len(results),
		"results":   results,
	}

	jsonData, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		fmt.Printf("[pioz2] 生成JSON失败: %v\n", err)
		return
	}

	err = os.WriteFile(filename, jsonData, 0644)
	if err != nil {
		fmt.Printf("[pioz2] 保存JSON文件失败: %v\n", err)
		return
	}

	fmt.Printf("[pioz2] 搜索结果已保存到: %s\n", filename)
}
