package main

import (
	"log"
	"pansou/plugin/pioz"
)

func main() {
	// 设置日志输出格式
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.Println("开始测试 Pioz 插件...")

	// 创建 Pioz 插件实例
	log.Println("创建 Pioz 插件实例...")
	plugin := pioz.NewPiozPlugin()
	log.Printf("插件创建成功: %s\n", plugin.Name())

	// 测试搜索功能
	keyword := "test"
	log.Printf("搜索关键词: '%s'\n", keyword)

	log.Println("开始搜索...")
	results, err := plugin.Search(keyword, nil)
	if err != nil {
		log.Printf("搜索失败: %v\n", err)
		return
	}

	log.Printf("搜索完成，找到 %d 个结果\n", len(results))

	// 输出每个结果
	for i, result := range results {
		log.Printf("结果 %d: %s\n", i+1, result.Title)
		log.Printf("  内容: %s\n", result.Content)
		log.Printf("  时间: %v\n", result.Datetime)
		log.Printf("  链接数: %d\n", len(result.Links))
		for j, link := range result.Links {
			log.Printf("    链接 %d: %s (%s)\n", j+1, link.URL, link.Type)
		}
		log.Println()
	}

	log.Println("测试完成！")
}
