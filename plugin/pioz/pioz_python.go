package pioz

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	"pansou/model"
	"pansou/plugin"
)

type PiozPythonPlugin struct {
	*plugin.BaseAsyncPlugin
	pythonPath  string
	scriptPath  string
	cache       sync.Map
	cacheTTL    time.Duration
	timeout     time.Duration
	maxResults  int
}

type pythonSearchResult struct {
	UniqueID string       `json:"unique_id"`
	Title    string       `json:"title"`
	Content  string       `json:"content"`
	Links    []model.Link `json:"links"`
	Channel  string       `json:"channel"`
	Tags     []string     `json:"tags"`
	Images   []string     `json:"images"`
}

type cacheItem struct {
	results   []model.SearchResult
	timestamp time.Time
}

func NewPiozPythonPlugin() *PiozPythonPlugin {
	pythonPath := findPython()
	scriptPath := findScriptPath()
	
	return &PiozPythonPlugin{
		BaseAsyncPlugin: plugin.NewBaseAsyncPlugin("pioz", 1),
		pythonPath:      pythonPath,
		scriptPath:      scriptPath,
		cacheTTL:        30 * time.Minute,
		timeout:         30 * time.Second,
		maxResults:      20,
	}
}

func findPython() string {
	candidates := []string{"python3", "python"}
	if runtime.GOOS == "windows" {
		candidates = []string{"python", "python3", "py"}
	}
	
	for _, cmd := range candidates {
		if path, err := exec.LookPath(cmd); err == nil {
			return path
		}
	}
	return "python"
}

func findScriptPath() string {
	_, filename, _, _ := runtime.Caller(0)
	dir := filepath.Dir(filename)
	scriptPath := filepath.Join(dir, "pioz.py")
	
	if _, err := os.Stat(scriptPath); err == nil {
		return scriptPath
	}
	
	return "pioz.py"
}

func (p *PiozPythonPlugin) Search(keyword string, ext map[string]interface{}) ([]model.SearchResult, error) {
	result, err := p.SearchWithResult(keyword, ext)
	if err != nil {
		return nil, err
	}
	return result.Results, nil
}

func (p *PiozPythonPlugin) SearchWithResult(keyword string, ext map[string]interface{}) (model.PluginSearchResult, error) {
	return p.AsyncSearchWithResult(keyword, p.searchImpl, p.MainCacheKey, ext)
}

func (p *PiozPythonPlugin) searchImpl(client *http.Client, keyword string, ext map[string]interface{}) ([]model.SearchResult, error) {
	if cached, ok := p.cache.Load(keyword); ok {
		if item, ok := cached.(cacheItem); ok {
			if time.Since(item.timestamp) < p.cacheTTL {
				return item.results, nil
			}
		}
	}
	
	results, err := p.executePythonScript(keyword)
	if err != nil {
		return nil, fmt.Errorf("[pioz] Python脚本执行失败: %w", err)
	}
	
	if len(results) > p.maxResults {
		results = results[:p.maxResults]
	}
	
	p.cache.Store(keyword, cacheItem{
		results:   results,
		timestamp: time.Now(),
	})
	
	fmt.Printf("[pioz] Python插件搜索完成: %s -> %d 结果\n", keyword, len(results))
	return results, nil
}

func (p *PiozPythonPlugin) executePythonScript(keyword string) ([]model.SearchResult, error) {
	if p.pythonPath == "" {
		return nil, fmt.Errorf("未找到Python解释器")
	}
	
	ctx, cancel := context.WithTimeout(context.Background(), p.timeout)
	defer cancel()
	
	cmd := exec.CommandContext(ctx, p.pythonPath, p.scriptPath, keyword)
	
	output, err := cmd.Output()
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return nil, fmt.Errorf("Python脚本执行超时")
		}
		return nil, fmt.Errorf("执行失败: %w", err)
	}
	
	var pythonResults []pythonSearchResult
	if err := json.Unmarshal(output, &pythonResults); err != nil {
		return nil, fmt.Errorf("解析JSON失败: %w, output: %s", err, string(output))
	}
	
	results := make([]model.SearchResult, 0, len(pythonResults))
	for _, pr := range pythonResults {
		results = append(results, model.SearchResult{
			UniqueID: pr.UniqueID,
			Title:    pr.Title,
			Content:  pr.Content,
			Links:    pr.Links,
			Channel:  pr.Channel,
			Tags:     pr.Tags,
			Images:   pr.Images,
		})
	}
	
	return results, nil
}

func (p *PiozPythonPlugin) IsPythonAvailable() bool {
	return p.pythonPath != "" && p.scriptPath != ""
}
