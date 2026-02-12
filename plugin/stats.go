package plugin

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// PluginStats 插件统计信息
type PluginStats struct {
	PluginName      string    `json:"plugin_name"`
	TotalSearches   int64     `json:"total_searches"`    // 总搜索次数
	TotalResults    int64     `json:"total_results"`     // 总结果数
	SuccessSearches int64     `json:"success_searches"`  // 成功搜索次数（有结果）
	FailedSearches  int64     `json:"failed_searches"`   // 失败搜索次数
	AvgResults      float64   `json:"avg_results"`       // 平均结果数
	AvgResponseTime float64   `json:"avg_response_time"` // 平均响应时间（毫秒）
	LastSearchTime  time.Time `json:"last_search_time"`  // 最后搜索时间
	CustomPriority  int       `json:"custom_priority"`   // 自定义优先级（0表示使用默认）
}

// GlobalPluginStatsManager 全局插件统计管理器
type GlobalPluginStatsManager struct {
	stats     map[string]*PluginStats
	statsLock sync.RWMutex
	statsFile string
}

var (
	globalStatsManager *GlobalPluginStatsManager
	statsManagerOnce   sync.Once
)

// GetGlobalStatsManager 获取全局统计管理器
func GetGlobalStatsManager() *GlobalPluginStatsManager {
	statsManagerOnce.Do(func() {
		// 确定统计文件路径
		statsFile := os.Getenv("PLUGIN_STATS_FILE")
		if statsFile == "" {
			statsFile = "./cache/plugin_stats.json"
		}

		globalStatsManager = &GlobalPluginStatsManager{
			stats:     make(map[string]*PluginStats),
			statsFile: statsFile,
		}

		// 加载已有统计数据
		globalStatsManager.Load()
	})
	return globalStatsManager
}

// RecordSearch 记录一次搜索
func (m *GlobalPluginStatsManager) RecordSearch(pluginName string, resultCount int, responseTime time.Duration, success bool) {
	m.statsLock.Lock()
	defer m.statsLock.Unlock()

	stats, exists := m.stats[pluginName]
	if !exists {
		stats = &PluginStats{
			PluginName: pluginName,
		}
		m.stats[pluginName] = stats
	}

	stats.TotalSearches++
	stats.LastSearchTime = time.Now()

	if success {
		stats.SuccessSearches++
		stats.TotalResults += int64(resultCount)

		// 更新平均结果数
		stats.AvgResults = float64(stats.TotalResults) / float64(stats.SuccessSearches)
	} else {
		stats.FailedSearches++
	}

	// 更新平均响应时间
	if stats.TotalSearches == 1 {
		stats.AvgResponseTime = float64(responseTime.Milliseconds())
	} else {
		// 使用移动平均
		stats.AvgResponseTime = (stats.AvgResponseTime*float64(stats.TotalSearches-1) + float64(responseTime.Milliseconds())) / float64(stats.TotalSearches)
	}
}

// GetStats 获取插件统计信息
func (m *GlobalPluginStatsManager) GetStats(pluginName string) *PluginStats {
	m.statsLock.RLock()
	defer m.statsLock.RUnlock()

	if stats, exists := m.stats[pluginName]; exists {
		// 返回副本
		statsCopy := *stats
		return &statsCopy
	}
	return nil
}

// GetAllStats 获取所有插件统计信息
func (m *GlobalPluginStatsManager) GetAllStats() map[string]*PluginStats {
	m.statsLock.RLock()
	defer m.statsLock.RUnlock()

	result := make(map[string]*PluginStats)
	for name, stats := range m.stats {
		statsCopy := *stats
		result[name] = &statsCopy
	}
	return result
}

// SetCustomPriority 设置自定义优先级
func (m *GlobalPluginStatsManager) SetCustomPriority(pluginName string, priority int) error {
	m.statsLock.Lock()
	defer m.statsLock.Unlock()

	stats, exists := m.stats[pluginName]
	if !exists {
		stats = &PluginStats{
			PluginName: pluginName,
		}
		m.stats[pluginName] = stats
	}

	stats.CustomPriority = priority

	// 保存到文件
	return m.saveUnlocked()
}

// GetCustomPriority 获取自定义优先级
func (m *GlobalPluginStatsManager) GetCustomPriority(pluginName string) int {
	m.statsLock.RLock()
	defer m.statsLock.RUnlock()

	if stats, exists := m.stats[pluginName]; exists {
		return stats.CustomPriority
	}
	return 0 // 0表示使用默认优先级
}

// Save 保存统计数据到文件
func (m *GlobalPluginStatsManager) Save() error {
	m.statsLock.Lock()
	defer m.statsLock.Unlock()
	return m.saveUnlocked()
}

// saveUnlocked 保存统计数据（不加锁版本）
func (m *GlobalPluginStatsManager) saveUnlocked() error {
	// 确保目录存在
	dir := filepath.Dir(m.statsFile)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("创建统计目录失败: %w", err)
	}

	// 序列化数据
	data, err := json.MarshalIndent(m.stats, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化统计数据失败: %w", err)
	}

	// 写入文件
	if err := os.WriteFile(m.statsFile, data, 0644); err != nil {
		return fmt.Errorf("写入统计文件失败: %w", err)
	}

	return nil
}

// Load 从文件加载统计数据
func (m *GlobalPluginStatsManager) Load() error {
	m.statsLock.Lock()
	defer m.statsLock.Unlock()

	// 检查文件是否存在
	if _, err := os.Stat(m.statsFile); os.IsNotExist(err) {
		return nil // 文件不存在，不是错误
	}

	// 读取文件
	data, err := os.ReadFile(m.statsFile)
	if err != nil {
		return fmt.Errorf("读取统计文件失败: %w", err)
	}

	// 反序列化
	if err := json.Unmarshal(data, &m.stats); err != nil {
		return fmt.Errorf("解析统计数据失败: %w", err)
	}

	return nil
}

// PrintStats 打印统计信息
func (m *GlobalPluginStatsManager) PrintStats() {
	m.statsLock.RLock()
	defer m.statsLock.RUnlock()

	fmt.Println("\n========== 插件搜索效能统计 ==========")
	fmt.Printf("%-15s | %8s | %8s | %8s | %8s | %10s | %8s | %8s\n",
		"插件名称", "搜索次数", "成功次数", "总结果数", "平均结果", "平均响应(ms)", "成功率(%)", "自定义优先级")
	fmt.Println("----------------------------------------------------------------------------------------")

	for _, stats := range m.stats {
		successRate := float64(0)
		if stats.TotalSearches > 0 {
			successRate = float64(stats.SuccessSearches) / float64(stats.TotalSearches) * 100
		}

		priorityStr := "-"
		if stats.CustomPriority > 0 {
			priorityStr = fmt.Sprintf("%d", stats.CustomPriority)
		}

		fmt.Printf("%-15s | %8d | %8d | %8d | %8.1f | %10.1f | %8.1f | %8s\n",
			stats.PluginName,
			stats.TotalSearches,
			stats.SuccessSearches,
			stats.TotalResults,
			stats.AvgResults,
			stats.AvgResponseTime,
			successRate,
			priorityStr)
	}
	fmt.Println("================================================================================")
}

// StartAutoSave 启动自动保存
func (m *GlobalPluginStatsManager) StartAutoSave(interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for range ticker.C {
			if err := m.Save(); err != nil {
				fmt.Printf("[统计] 自动保存失败: %v\n", err)
			}
		}
	}()
}
