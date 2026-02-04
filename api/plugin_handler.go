package api

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"pansou/plugin"
)

// GetPluginStatsHandler 获取插件统计信息
func GetPluginStatsHandler(c *gin.Context) {
	statsManager := plugin.GetGlobalStatsManager()
	allStats := statsManager.GetAllStats()
	
	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "success",
		"data":    allStats,
	})
}

// GetPluginStatsDetailHandler 获取单个插件的统计信息
func GetPluginStatsDetailHandler(c *gin.Context) {
	pluginName := c.Param("name")
	if pluginName == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "插件名称不能为空",
		})
		return
	}
	
	statsManager := plugin.GetGlobalStatsManager()
	stats := statsManager.GetStats(pluginName)
	
	if stats == nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": "插件不存在或无统计数据",
		})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "success",
		"data":    stats,
	})
}

// SetPluginPriorityRequest 设置插件优先级请求
type SetPluginPriorityRequest struct {
	PluginName string `json:"plugin_name" binding:"required"`
	Priority   int    `json:"priority" binding:"required,min=1,max=10"`
}

// SetPluginPriorityHandler 设置插件优先级
func SetPluginPriorityHandler(c *gin.Context) {
	var req SetPluginPriorityRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "请求参数错误: " + err.Error(),
		})
		return
	}
	
	// 检查插件是否存在
	if _, exists := plugin.GetPluginByName(req.PluginName); !exists {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": "插件不存在: " + req.PluginName,
		})
		return
	}
	
	// 设置优先级
	statsManager := plugin.GetGlobalStatsManager()
	if err := statsManager.SetCustomPriority(req.PluginName, req.Priority); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "设置优先级失败: " + err.Error(),
		})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "优先级设置成功",
		"data": gin.H{
			"plugin_name": req.PluginName,
			"priority":    req.Priority,
		},
	})
}

// ResetPluginPriorityHandler 重置插件优先级（恢复默认）
func ResetPluginPriorityHandler(c *gin.Context) {
	pluginName := c.Param("name")
	if pluginName == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "插件名称不能为空",
		})
		return
	}
	
	// 检查插件是否存在
	pluginInstance, exists := plugin.GetPluginByName(pluginName)
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": "插件不存在: " + pluginName,
		})
		return
	}
	
	// 设置优先级为0（表示使用默认）
	statsManager := plugin.GetGlobalStatsManager()
	if err := statsManager.SetCustomPriority(pluginName, 0); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "重置优先级失败: " + err.Error(),
		})
		return
	}
	
	// 获取默认优先级（需要通过反射或类型断言）
	defaultPriority := 3 // 默认值
	if basePlugin, ok := pluginInstance.(*plugin.BaseAsyncPlugin); ok {
		// 这里无法直接访问priority字段，因为它是私有的
		// 但我们可以通过其他方式获取
		defaultPriority = basePlugin.Priority()
	}
	
	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "优先级已重置为默认值",
		"data": gin.H{
			"plugin_name":      pluginName,
			"default_priority": defaultPriority,
		},
	})
}

// BatchSetPluginPriorityRequest 批量设置插件优先级请求
type BatchSetPluginPriorityRequest struct {
	Priorities map[string]int `json:"priorities" binding:"required"`
}

// BatchSetPluginPriorityHandler 批量设置插件优先级
func BatchSetPluginPriorityHandler(c *gin.Context) {
	var req BatchSetPluginPriorityRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "请求参数错误: " + err.Error(),
		})
		return
	}
	
	statsManager := plugin.GetGlobalStatsManager()
	successCount := 0
	failedPlugins := make([]string, 0)
	
	for pluginName, priority := range req.Priorities {
		// 验证优先级范围
		if priority < 0 || priority > 10 {
			failedPlugins = append(failedPlugins, pluginName+" (优先级超出范围)")
			continue
		}
		
		// 检查插件是否存在
		if _, exists := plugin.GetPluginByName(pluginName); !exists {
			failedPlugins = append(failedPlugins, pluginName+" (插件不存在)")
			continue
		}
		
		// 设置优先级
		if err := statsManager.SetCustomPriority(pluginName, priority); err != nil {
			failedPlugins = append(failedPlugins, pluginName+" (设置失败)")
			continue
		}
		
		successCount++
	}
	
	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "批量设置完成",
		"data": gin.H{
			"success_count": successCount,
			"failed_count":  len(failedPlugins),
			"failed_plugins": failedPlugins,
		},
	})
}

// GetAllPluginsHandler 获取所有插件信息（包括优先级）
func GetAllPluginsHandler(c *gin.Context) {
	allPlugins := plugin.GetRegisteredPlugins()
	statsManager := plugin.GetGlobalStatsManager()
	
	type PluginInfo struct {
		Name            string  `json:"name"`
		DefaultPriority int     `json:"default_priority"`
		CustomPriority  int     `json:"custom_priority"`
		CurrentPriority int     `json:"current_priority"`
		TotalSearches   int64   `json:"total_searches"`
		AvgResults      float64 `json:"avg_results"`
		AvgResponseTime float64 `json:"avg_response_time"`
	}
	
	pluginInfos := make([]PluginInfo, 0, len(allPlugins))
	for _, p := range allPlugins {
		customPriority := statsManager.GetCustomPriority(p.Name())
		currentPriority := p.Priority()
		
		// 获取统计信息
		stats := statsManager.GetStats(p.Name())
		totalSearches := int64(0)
		avgResults := float64(0)
		avgResponseTime := float64(0)
		
		if stats != nil {
			totalSearches = stats.TotalSearches
			avgResults = stats.AvgResults
			avgResponseTime = stats.AvgResponseTime
		}
		
		pluginInfos = append(pluginInfos, PluginInfo{
			Name:            p.Name(),
			DefaultPriority: currentPriority, // 如果有自定义优先级，这里会显示自定义的
			CustomPriority:  customPriority,
			CurrentPriority: currentPriority,
			TotalSearches:   totalSearches,
			AvgResults:      avgResults,
			AvgResponseTime: avgResponseTime,
		})
	}
	
	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "success",
		"data":    pluginInfos,
	})
}

// ExportPluginStatsHandler 导出插件统计数据
func ExportPluginStatsHandler(c *gin.Context) {
	format := c.DefaultQuery("format", "json")
	
	statsManager := plugin.GetGlobalStatsManager()
	allStats := statsManager.GetAllStats()
	
	switch format {
	case "json":
		c.JSON(http.StatusOK, allStats)
	case "csv":
		// 生成CSV格式
		c.Header("Content-Type", "text/csv")
		c.Header("Content-Disposition", "attachment; filename=plugin_stats.csv")
		
		// CSV头部
		c.String(http.StatusOK, "插件名称,搜索次数,成功次数,总结果数,平均结果,平均响应时间(ms),自定义优先级\n")
		
		// CSV数据
		for _, stats := range allStats {
			priorityStr := strconv.Itoa(stats.CustomPriority)
			if stats.CustomPriority == 0 {
				priorityStr = "-"
			}
			c.String(http.StatusOK, "%s,%d,%d,%d,%.1f,%.1f,%s\n",
				stats.PluginName,
				stats.TotalSearches,
				stats.SuccessSearches,
				stats.TotalResults,
				stats.AvgResults,
				stats.AvgResponseTime,
				priorityStr)
		}
	default:
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "不支持的格式: " + format,
		})
	}
}
