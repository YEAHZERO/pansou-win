package main

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"pansou/plugin/pioz"
)

func main() {
	fmt.Println("开始直接测试 Pioz 插件的搜索功能...")

	// 创建 Pioz 插件实例
	plugin := pioz.NewPiozPlugin()
	fmt.Printf("插件创建成功: %s\n", plugin.Name())

	// 测试搜索功能
	keyword := "test"
	fmt.Printf("搜索关键词: '%s'\n", keyword)

	// 直接访问搜索页面，查看页面结构
	searchURL := fmt.Sprintf("%s/search?q=%s", pioz.SiteBaseURL, url.QueryEscape(keyword))
	fmt.Printf("访问搜索页面: %s\n", searchURL)

	// 创建HTTP客户端
	client := &http.Client{
		Timeout: 15 * time.Second,
	}

	// 创建请求
	req, err := http.NewRequest("GET", searchURL, nil)
	if err != nil {
		fmt.Printf("创建请求失败: %v\n", err)
		return
	}

	// 设置请求头
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.7")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8,en-GB;q=0.7,en-US;q=0.6")
	req.Header.Set("Accept-Encoding", "gzip, deflate, br")
	req.Header.Set("Connection", "keep-alive")
	req.Header.Set("Upgrade-Insecure-Requests", "1")
	req.Header.Set("Referer", "https://www.pioz.cn/")

	// 发送请求
	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("请求失败: %v\n", err)
		return
	}
	defer resp.Body.Close()

	// 读取响应内容
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Printf("读取响应失败: %v\n", err)
		return
	}

	pageContent := string(body)
	fmt.Printf("页面内容长度: %d 字节\n", len(pageContent))

	// 输出页面的前2000个字符，以便了解页面结构
	previewLength := 2000
	if len(pageContent) < previewLength {
		previewLength = len(pageContent)
	}
	fmt.Println("页面内容预览:")
	fmt.Println(pageContent[:previewLength])

	// 检查页面是否包含搜索结果
	if strings.Contains(pageContent, "results") {
		fmt.Println("\n页面包含 'results' 字段")
		// 输出包含 results 的部分
		if idx := strings.Index(pageContent, "results"); idx > 0 {
			endIdx := idx + 500
			if endIdx > len(pageContent) {
				endIdx = len(pageContent)
			}
			fmt.Printf("results 上下文: %s\n", pageContent[idx:endIdx])
		}
	}

	// 检查页面是否包含其他可能的关键词
	if strings.Contains(pageContent, "search") {
		fmt.Println("页面包含 'search' 字段")
	}
	if strings.Contains(pageContent, "item") {
		fmt.Println("页面包含 'item' 字段")
	}
	if strings.Contains(pageContent, "detail") {
		fmt.Println("页面包含 'detail' 字段")
	}

	// 尝试使用插件的 Search 方法搜索结果
	fmt.Println("\n尝试使用插件的 Search 方法搜索结果...")
	results, err := plugin.Search(keyword, nil)
	if err != nil {
		fmt.Printf("搜索失败: %v\n", err)
		return
	}
	fmt.Printf("搜索找到 %d 个结果\n", len(results))

	// 输出每个结果
	for i, result := range results {
		fmt.Printf("结果 %d: %s\n", i+1, result.Title)
		fmt.Printf("  内容: %s\n", result.Content)
		fmt.Printf("  时间: %v\n", result.Datetime)
		fmt.Printf("  链接数: %d\n", len(result.Links))
		for j, link := range result.Links {
			fmt.Printf("    链接 %d: %s (%s)\n", j+1, link.URL, link.Type)
		}
		fmt.Println()
	}

	fmt.Println("测试完成！")
}
