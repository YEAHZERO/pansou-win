package cache

import (
	"crypto/md5"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"pansou/model"
)

type CacheWriteStrategy int

const (
	WriteImmediately CacheWriteStrategy = iota
	DelayedWrite
	HybridWrite
)

type CacheOperation struct {
	Key        string
	Data       interface{}
	TTL        time.Duration
	PluginName string
	Keyword    string
	Timestamp  time.Time
	Priority   int
	DataSize   int
	IsFinal    bool
}

type CacheSerializer interface {
	Serialize(data interface{}) ([]byte, error)
	Deserialize(data []byte, target interface{}) error
}

type JSONSerializer struct{}

func (s *JSONSerializer) Serialize(data interface{}) ([]byte, error) {
	return json.Marshal(data)
}

func (s *JSONSerializer) Deserialize(data []byte, target interface{}) error {
	return json.Unmarshal(data, target)
}

type EnhancedTwoLevelCache struct {
	memoryCache sync.Map
	diskPath    string
	maxSize     int64
	ttl         time.Duration
	serializer  CacheSerializer
	mu          sync.RWMutex
}

func NewEnhancedTwoLevelCache() (*EnhancedTwoLevelCache, error) {
	cachePath := "./cache"
	if err := os.MkdirAll(cachePath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create cache directory: %w", err)
	}

	return &EnhancedTwoLevelCache{
		diskPath:   cachePath,
		maxSize:    100 * 1024 * 1024,
		ttl:        60 * time.Minute,
		serializer: &JSONSerializer{},
	}, nil
}

func (c *EnhancedTwoLevelCache) Get(key string) ([]byte, bool, error) {
	if val, ok := c.memoryCache.Load(key); ok {
		if data, ok := val.([]byte); ok {
			return data, true, nil
		}
	}

	diskPath := c.getDiskPath(key)
	data, err := os.ReadFile(diskPath)
	if err == nil {
		c.memoryCache.Store(key, data)
		return data, true, nil
	}

	return nil, false, nil
}

func (c *EnhancedTwoLevelCache) Set(key string, data []byte, ttl time.Duration) error {
	c.memoryCache.Store(key, data)
	return c.writeToDisk(key, data)
}

func (c *EnhancedTwoLevelCache) SetMemoryOnly(key string, data []byte, ttl time.Duration) error {
	c.memoryCache.Store(key, data)
	return nil
}

func (c *EnhancedTwoLevelCache) SetBothLevels(key string, data []byte, ttl time.Duration) error {
	c.memoryCache.Store(key, data)
	return c.writeToDisk(key, data)
}

func (c *EnhancedTwoLevelCache) writeToDisk(key string, data []byte) error {
	diskPath := c.getDiskPath(key)
	return os.WriteFile(diskPath, data, 0644)
}

func (c *EnhancedTwoLevelCache) getDiskPath(key string) string {
	hash := md5.Sum([]byte(key))
	return filepath.Join(c.diskPath, fmt.Sprintf("%x.cache", hash))
}

func (c *EnhancedTwoLevelCache) GetSerializer() CacheSerializer {
	return c.serializer
}

func (c *EnhancedTwoLevelCache) FlushMemoryToDisk() error {
	var err error
	c.memoryCache.Range(func(key, value interface{}) bool {
		if k, ok := key.(string); ok {
			if v, ok := value.([]byte); ok {
				if writeErr := c.writeToDisk(k, v); writeErr != nil {
					err = writeErr
				}
			}
		}
		return true
	})
	return err
}

type DelayedBatchWriteManager struct {
	operations   chan *CacheOperation
	mainCache    func(string, []byte, time.Duration) error
	serializer   CacheSerializer
	initialized  bool
	shutdownChan chan struct{}
	wg           sync.WaitGroup
}

func NewDelayedBatchWriteManager() (*DelayedBatchWriteManager, error) {
	return &DelayedBatchWriteManager{
		operations:   make(chan *CacheOperation, 1000),
		serializer:   &JSONSerializer{},
		shutdownChan: make(chan struct{}),
	}, nil
}

func (m *DelayedBatchWriteManager) Initialize() error {
	m.initialized = true
	m.wg.Add(1)
	go m.processBatch()
	return nil
}

func (m *DelayedBatchWriteManager) processBatch() {
	defer m.wg.Done()

	for {
		select {
		case op := <-m.operations:
			if op == nil {
				continue
			}
			m.processOperation(op)
		case <-m.shutdownChan:
			for {
				select {
				case op := <-m.operations:
					if op != nil {
						m.processOperation(op)
					}
				default:
					return
				}
			}
		}
	}
}

func (m *DelayedBatchWriteManager) processOperation(op *CacheOperation) {
	if m.mainCache == nil {
		return
	}

	data, err := m.serializer.Serialize(op.Data)
	if err != nil {
		return
	}

	m.mainCache(op.Key, data, op.TTL)
}

func (m *DelayedBatchWriteManager) HandleCacheOperation(op *CacheOperation) error {
	if !m.initialized {
		return fmt.Errorf("manager not initialized")
	}

	select {
	case m.operations <- op:
	default:
		go func() {
			m.operations <- op
		}()
	}

	return nil
}

func (m *DelayedBatchWriteManager) SetMainCacheUpdater(updater func(string, []byte, time.Duration) error) {
	m.mainCache = updater
}

func (m *DelayedBatchWriteManager) Shutdown(timeout time.Duration) error {
	close(m.shutdownChan)

	done := make(chan struct{})
	go func() {
		m.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-time.After(timeout):
		return fmt.Errorf("shutdown timeout")
	}
}

func (m *DelayedBatchWriteManager) GetStats() interface{} {
	return map[string]interface{}{
		"queue_size": len(m.operations),
	}
}

func GenerateTGCacheKey(keyword string, channels []string) string {
	key := "tg:" + keyword
	for _, ch := range channels {
		key += ":" + ch
	}
	return key
}

func GeneratePluginCacheKey(keyword string, plugins []string) string {
	key := "plugin:" + keyword

	normalized := make([]string, 0, len(plugins))
	for _, p := range plugins {
		name := strings.ToLower(strings.TrimSpace(p))
		if name == "" {
			continue
		}
		normalized = append(normalized, name)
	}

	if len(normalized) == 0 {
		return key + ":all"
	}

	sort.Strings(normalized)
	for _, p := range normalized {
		key += ":" + p
	}
	return key
}

func MergeSearchResults(existing []model.SearchResult, newResults []model.SearchResult) []model.SearchResult {
	resultMap := make(map[string]model.SearchResult)

	for _, r := range existing {
		resultMap[r.UniqueID] = r
	}

	for _, r := range newResults {
		if existing, ok := resultMap[r.UniqueID]; ok {
			if len(r.Links) > len(existing.Links) {
				resultMap[r.UniqueID] = r
			}
		} else {
			resultMap[r.UniqueID] = r
		}
	}

	results := make([]model.SearchResult, 0, len(resultMap))
	for _, r := range resultMap {
		results = append(results, r)
	}

	return results
}
