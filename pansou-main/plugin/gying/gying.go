package gying

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unsafe"

	"github.com/gin-gonic/gin"
	"pansou/model"
	"pansou/plugin"
	"pansou/util/json"
	
	cloudscraper "github.com/Advik-B/cloudscraper/lib"
)

// 插件配置参数
const (
	MaxConcurrentUsers = 10    // 最多使用的用户数
	MaxConcurrentDetails = 50  // 最大并发详情请求数
	DebugLog = false           // 调试日志开关（排查问题时改为true）
)

// 默认账户配置（可通过Web界面添加更多账户）
// 用户数据会保存到文件，重启后自动恢复
var DefaultAccounts = []struct {
	Username string
	Password string
}{
	// 请使用 Web 接口添加用户：
	// POST /gying/add_user?username=xxx&password=xxx
}

// 存储目录
var StorageDir string

// 初始化存储目录

// HTML模板
const HTMLTemplate = `<!DOCTYPE html>
<html lang="zh-CN">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>PanSou Gying搜索配置</title>
    <style>
        * { margin: 0; padding: 0; box-sizing: border-box; }
        body { 
            font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif;
            background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
            padding: 20px;
        }
        .container {
            max-width: 800px;
            margin: 0 auto;
            background: white;
            border-radius: 16px;
            box-shadow: 0 20px 60px rgba(0,0,0,0.3);
            overflow: hidden;
        }
        .header {
            background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
            color: white;
            padding: 30px;
            text-align: center;
        }
        .section {
            padding: 30px;
            border-bottom: 1px solid #eee;
        }
        .section:last-child { border-bottom: none; }
        .section-title {
            font-size: 18px;
            font-weight: bold;
            margin-bottom: 15px;
            color: #333;
        }
        .status-box {
            background: #f8f9fa;
            padding: 20px;
            border-radius: 8px;
            margin-bottom: 15px;
        }
        .status-item {
            display: flex;
            justify-content: space-between;
            padding: 8px 0;
        }
        .form-group {
            margin-bottom: 15px;
        }
        .form-group label {
            display: block;
            margin-bottom: 5px;
            font-weight: bold;
        }
        .form-group input {
            width: 100%;
            padding: 10px;
            border: 1px solid #ddd;
            border-radius: 6px;
        }
        .btn {
            padding: 10px 20px;
            border: none;
            border-radius: 6px;
            cursor: pointer;
            font-size: 14px;
            transition: all 0.3s;
        }
        .btn-primary {
            background: #667eea;
            color: white;
        }
        .btn-primary:hover { background: #5568d3; }
        .btn-danger {
            background: #f56565;
            color: white;
        }
        .btn-danger:hover { background: #e53e3e; }
        .alert {
            padding: 12px 15px;
            border-radius: 6px;
            margin: 10px 0;
        }
        .alert-success {
            background: #c6f6d5;
            color: #22543d;
        }
        .alert-error {
            background: #fed7d7;
            color: #742a2a;
        }
        .test-results {
            max-height: 300px;
            overflow-y: auto;
            background: #f8f9fa;
            padding: 15px;
            border-radius: 6px;
            margin-top: 10px;
        }
        .hidden { display: none; }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1>🔍 PanSou Gying搜索</h1>
            <p>配置你的专属搜索服务</p>
            <p style="font-size: 12px; margin-top: 10px; opacity: 0.8;">
                🔗 当前地址: <span id="current-url">HASH_PLACEHOLDER</span>
            </p>
        </div>

        <div class="section" id="login-section">
            <div class="section-title">🔐 登录状态</div>
            
            <div id="logged-in-view" class="hidden">
                <div class="status-box">
                    <div class="status-item">
                        <span>状态</span>
                        <span><strong style="color: #48bb78;">✅ 已登录</strong></span>
                    </div>
                    <div class="status-item">
                        <span>用户名</span>
                        <span id="username-display">-</span>
                    </div>
                    <div class="status-item">
                        <span>登录时间</span>
                        <span id="login-time">-</span>
                    </div>
                    <div class="status-item">
                        <span>有效期</span>
                        <span id="expire-info">-</span>
                    </div>
                </div>
                <button class="btn btn-danger" onclick="logout()">退出登录</button>
            </div>

            <div id="not-logged-in-view" class="hidden">
                <div id="alert-box"></div>
                <div class="form-group">
                    <label>用户名</label>
                    <input type="text" id="username" placeholder="输入用户名">
                </div>
                <div class="form-group">
                    <label>密码</label>
                    <input type="password" id="password" placeholder="输入密码">
                </div>
                <button class="btn btn-primary" onclick="login()">登录</button>
            </div>
        </div>

        <div class="section" id="test-section">
            <div class="section-title">🔍 测试搜索(限制返回10条数据)</div>
            
            <div style="display: flex; gap: 10px;">
                <input type="text" id="search-keyword" placeholder="输入关键词测试搜索" style="flex: 1; padding: 10px; border: 1px solid #ddd; border-radius: 6px;">
                <button class="btn btn-primary" onclick="testSearch()">搜索</button>
            </div>

            <div id="search-results" class="test-results hidden"></div>
        </div>

        <div class="section">
            <div class="section-title">📖 API调用说明</div>
            
            <p style="margin-bottom: 15px;">你可以通过API程序化管理：</p>

            <details>
                <summary style="cursor: pointer; padding: 10px 0; font-weight: bold;">登录</summary>
                <div style="background: #2d3748; color: #68d391; padding: 10px; border-radius: 6px; font-family: monospace; font-size: 12px; overflow-x: auto;">curl -X POST https://your-domain.com/gying/HASH_PLACEHOLDER \
  -H "Content-Type: application/json" \
  -d '{"action": "login", "username": "user", "password": "pass"}'</div>
            </details>
        </div>
    </div>

    <script>
        const HASH = 'HASH_PLACEHOLDER';
        const API_URL = '/gying/' + HASH;
        let statusCheckInterval = null;

        window.onload = function() {
            updateStatus();
            startStatusPolling();
        };

        function startStatusPolling() {
            statusCheckInterval = setInterval(updateStatus, 5000);
        }

        async function postAction(action, extraData = {}) {
            try {
                const response = await fetch(API_URL, {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({ action: action, ...extraData })
                });
                return await response.json();
            } catch (error) {
                console.error('请求失败:', error);
                return { success: false, message: '请求失败: ' + error.message };
            }
        }

        async function updateStatus() {
            const result = await postAction('get_status');
            if (result.success && result.data) {
                const data = result.data;
                
                if (data.logged_in === true && data.status === 'active') {
                    document.getElementById('logged-in-view').classList.remove('hidden');
                    document.getElementById('not-logged-in-view').classList.add('hidden');
                    
                    document.getElementById('username-display').textContent = data.username_masked || '-';
                    document.getElementById('login-time').textContent = data.login_time || '-';
                    document.getElementById('expire-info').textContent = '剩余 ' + (data.expires_in_days || 0) + ' 天';
                } else {
                    document.getElementById('logged-in-view').classList.add('hidden');
                    document.getElementById('not-logged-in-view').classList.remove('hidden');
                }
            }
        }

        function showAlert(message, type = 'success') {
            const alertBox = document.getElementById('alert-box');
            alertBox.innerHTML = '<div class="alert alert-' + type + '">' + message + '</div>';
            setTimeout(() => {
                alertBox.innerHTML = '';
            }, 3000);
        }

        async function login() {
            const username = document.getElementById('username').value.trim();
            const password = document.getElementById('password').value.trim();
            
            if (!username || !password) {
                showAlert('请输入用户名和密码', 'error');
                return;
            }

            const result = await postAction('login', { username, password });
            if (result.success) {
                showAlert(result.message);
                updateStatus();
            } else {
                showAlert(result.message, 'error');
            }
        }

        async function logout() {
            if (!confirm('确定要退出登录吗？')) return;
            
            const result = await postAction('logout');
            if (result.success) {
                showAlert(result.message);
                updateStatus();
            } else {
                showAlert(result.message, 'error');
            }
        }

        async function testSearch() {
            const keyword = document.getElementById('search-keyword').value.trim();
            
            if (!keyword) {
                showAlert('请输入搜索关键词', 'error');
                return;
            }

            const resultsDiv = document.getElementById('search-results');
            resultsDiv.classList.remove('hidden');
            resultsDiv.innerHTML = '<div>🔍 搜索中...</div>';

            const result = await postAction('test_search', { keyword });
            
            if (result.success) {
                const results = result.data.results || [];
                
                if (results.length === 0) {
                    resultsDiv.innerHTML = '<p style="text-align: center; color: #999;">未找到结果</p>';
                    return;
                }

                let html = '<p><strong>找到 ' + result.data.total_results + ' 条结果</strong></p>';
                results.forEach((item, index) => {
                    html += '<div style="margin: 15px 0; padding: 10px; background: white; border-radius: 6px;">';
                    html += '<p><strong>' + (index + 1) + '. ' + item.title + '</strong></p>';
                    item.links.forEach(link => {
                        html += '<p style="font-size: 12px; color: #666; margin: 5px 0; word-break: break-all;">';
                        html += '[' + link.type + '] ' + link.url;
                        if (link.password) html += ' 密码: ' + link.password;
                        html += '</p>';
                    });
                    html += '</div>';
                });
                resultsDiv.innerHTML = html;
            } else {
                resultsDiv.innerHTML = '<p style="color: red;">' + result.message + '</p>';
            }
        }

        document.getElementById('search-keyword').addEventListener('keypress', function(e) {
            if (e.key === 'Enter') testSearch();
        });
    </script>
</body>
</html>`

// GyingPlugin 插件结构
type GyingPlugin struct {
	*plugin.BaseAsyncPlugin
	users       sync.Map // 内存缓存：hash -> *User
	scrapers    sync.Map // cloudscraper实例缓存：hash -> *cloudscraper.Scraper
	mu          sync.RWMutex
	searchCache sync.Map // 插件级缓存：关键词->model.PluginSearchResult
	initialized bool     // 初始化状态标记
}

// User 用户数据结构
type User struct {
	Hash              string    `json:"hash"`
	Username          string    `json:"username"`           // 原始用户名（存储）
	UsernameMasked    string    `json:"username_masked"`    // 脱敏用户名（显示）
	EncryptedPassword string    `json:"encrypted_password"` // 加密后的密码（用于重启恢复）
	Cookie            string    `json:"cookie"`             // 登录Cookie字符串（仅供参考）
	Status            string    `json:"status"`             // pending/active/expired
	CreatedAt         time.Time `json:"created_at"`
	LoginAt           time.Time `json:"login_at"`
	ExpireAt          time.Time `json:"expire_at"`
	LastAccessAt      time.Time `json:"last_access_at"`
}

// SearchData 搜索页面JSON数据结构
type SearchData struct {
	Q  string   `json:"q"`  // 搜索关键词
	WD []string `json:"wd"` // 分词
	N  string   `json:"n"`  // 结果数量
	L  struct {
		Title  []string `json:"title"`  // 标题数组
		Year   []int    `json:"year"`   // 年份数组
		D      []string `json:"d"`      // 类型数组（mv/ac/tv）
		I      []string `json:"i"`      // 资源ID数组
		Info   []string `json:"info"`   // 信息数组
		Daoyan []string `json:"daoyan"` // 导演数组
		Zhuyan []string `json:"zhuyan"` // 主演数组
	} `json:"l"`
}

// DetailData 详情接口JSON数据结构
type DetailData struct {
	Code int  `json:"code"`
	WP   bool `json:"wp"`
	Panlist struct {
		ID    []string `json:"id"`
		Name  []string `json:"name"`
		P     []string `json:"p"`     // 提取码数组
		URL   []string `json:"url"`   // 链接数组
		Type  []int    `json:"type"`  // 类型标识
		User  []string `json:"user"`  // 分享用户
		Time  []string `json:"time"`  // 分享时间
		TName []string `json:"tname"` // 网盘类型名称
	} `json:"panlist"`
}

func init() {
	p := &GyingPlugin{
		BaseAsyncPlugin: plugin.NewBaseAsyncPlugin("gying", 3),
	}

	plugin.RegisterGlobalPlugin(p)
}

// Initialize 实现 InitializablePlugin 接口，延迟初始化插件
func (p *GyingPlugin) Initialize() error {
	if p.initialized {
		return nil
	}

	// 初始化存储目录路径
	cachePath := os.Getenv("CACHE_PATH")
	if cachePath == "" {
		cachePath = "./cache"
	}
	StorageDir = filepath.Join(cachePath, "gying_users")

	// 初始化存储目录
	if err := os.MkdirAll(StorageDir, 0755); err != nil {
		return fmt.Errorf("创建存储目录失败: %v", err)
	}

	// 加载所有用户到内存
	p.loadAllUsers()

	// 异步初始化默认账户（不阻塞启动）
	go func() {
		// 延迟1秒，等待主程序完全启动
		time.Sleep(1 * time.Second)
		p.initDefaultAccounts()
	}()

	// 启动定期清理任务
	go p.startCleanupTask()
	
	// 启动session保活任务（防止session超时）
	go p.startSessionKeepAlive()

	p.initialized = true
	return nil
}

// ============ 插件接口实现 ============

// RegisterWebRoutes 注册Web路由
func (p *GyingPlugin) RegisterWebRoutes(router *gin.RouterGroup) {
	gying := router.Group("/gying")
	gying.GET("/:param", p.handleManagePage)
	gying.POST("/:param", p.handleManagePagePOST)
	
	fmt.Printf("[Gying] Web路由已注册: /gying/:param\n")
}

// Search 执行搜索并返回结果
func (p *GyingPlugin) Search(keyword string, ext map[string]interface{}) ([]model.SearchResult, error) {
	result, err := p.SearchWithResult(keyword, ext)
	if err != nil {
		return nil, err
	}
	return result.Results, nil
}

// SearchWithResult 执行搜索并返回包含IsFinal标记的结果
// 注意：gying插件不使用AsyncSearchWithResult的缓存机制，因为：
// 1. 使用自己的cloudscraper实例而不是传入的http.Client
// 2. 有自己的用户会话管理
// 3. Service层已经有缓存，无需插件层再次缓存
func (p *GyingPlugin) SearchWithResult(keyword string, ext map[string]interface{}) (model.PluginSearchResult, error) {
    // 解析 ext["refresh"]
    forceRefresh := false
    if ext != nil {
        if v, ok := ext["refresh"]; ok {
            if b, ok := v.(bool); ok && b {
                forceRefresh = true
            }
        }
    }

    if !forceRefresh {
        if cacheItem, ok := p.searchCache.Load(keyword); ok {
            cached := cacheItem.(model.PluginSearchResult)
            if DebugLog {
                fmt.Printf("[Gying] 命中插件缓存: %s\n", keyword)
            }
            return cached, nil
        }
    } else {
        if DebugLog {
            fmt.Printf("[Gying] 强制刷新，此次跳过插件缓存，关键词: %s\n", keyword)
        }
    }

    // 原有真实抓取逻辑
    if DebugLog {
        fmt.Printf("[Gying] searchWithScraper REAL 执行: %s\n", keyword)
    }
    users := p.getActiveUsers()
    if DebugLog {
        fmt.Printf("[Gying] 找到 %d 个有效用户\n", len(users))
    }
    if len(users) == 0 {
        if DebugLog {
            fmt.Printf("[Gying] 没有有效用户，返回空结果\n")
        }
        return model.PluginSearchResult{Results: []model.SearchResult{}, IsFinal: true}, nil
    }
    if len(users) > MaxConcurrentUsers {
        sort.Slice(users, func(i, j int) bool {
            return users[i].LastAccessAt.After(users[j].LastAccessAt)
        })
        users = users[:MaxConcurrentUsers]
    }
    results := p.executeSearchTasks(users, keyword)
    if DebugLog {
        fmt.Printf("[Gying] 搜索完成，获得 %d 条结果\n", len(results))
    }
    realResult := model.PluginSearchResult{
        Results: results,
        IsFinal: true,
    }
    // 写入缓存
    if len(results) > 0 {
        p.searchCache.Store(keyword, realResult)
    }
    return realResult, nil
}

// ============ 用户管理 ============

// loadAllUsers 加载所有用户到内存（包括用户名、加密密码等）
// 注意：只加载用户数据，scraper实例将在initDefaultAccounts中使用密码重新登录获取
func (p *GyingPlugin) loadAllUsers() {
	files, err := ioutil.ReadDir(StorageDir)
	if err != nil {
		return
	}

	totalFiles := 0
	loadedCount := 0
	skippedInactive := 0
	
	for _, file := range files {
		if file.IsDir() || filepath.Ext(file.Name()) != ".json" {
			continue
		}
		
		totalFiles++

		filePath := filepath.Join(StorageDir, file.Name())
		data, err := ioutil.ReadFile(filePath)
		if err != nil {
			continue
		}

		var user User
		if err := json.Unmarshal(data, &user); err != nil {
			continue
		}

		// 过滤条件：status必须是active
		if user.Status != "active" {
			if DebugLog {
				fmt.Printf("[Gying] ⏭️  跳过用户 %s: status=%s (非active)\n", user.UsernameMasked, user.Status)
			}
			skippedInactive++
			continue
		}

		// 只存储用户数据（包括用户名和加密密码）
		// scraper实例将在initDefaultAccounts中通过重新登录获取
		p.users.Store(user.Hash, &user)
		loadedCount++
		
		if DebugLog {
			hasPassword := "无"
			if user.EncryptedPassword != "" {
				hasPassword = "有"
			}
			fmt.Printf("[Gying] ✅ 已加载用户 %s (密码:%s, 将在初始化时登录)\n", user.UsernameMasked, hasPassword)
		}
	}

	fmt.Printf("[Gying] 用户加载完成: 总文件=%d, 已加载=%d, 跳过(非active)=%d\n", 
		totalFiles, loadedCount, skippedInactive)
}

// initDefaultAccounts 初始化所有账户（异步执行，不阻塞启动）
// 包括：1. DefaultAccounts（代码配置）  2. 从文件加载的用户（使用加密密码重新登录）
func (p *GyingPlugin) initDefaultAccounts() {
	// fmt.Printf("[Gying] ========== 异步初始化所有账户 ==========\n")
	
	// 步骤1：处理DefaultAccounts（代码中配置的默认账户）
	for i, account := range DefaultAccounts {
		if DebugLog {
			fmt.Printf("[Gying] [默认账户 %d/%d] 处理: %s\n", i+1, len(DefaultAccounts), account.Username)
		}

		p.initOrRestoreUser(account.Username, account.Password, "default")
	}
	
	// 步骤2：遍历所有已加载的用户，恢复没有scraper的用户
	var usersToRestore []*User
	p.users.Range(func(key, value interface{}) bool {
		user := value.(*User)
		// 检查scraper是否存在
		_, scraperExists := p.scrapers.Load(user.Hash)
		if !scraperExists && user.EncryptedPassword != "" {
			usersToRestore = append(usersToRestore, user)
		}
		return true
	})
	
	if len(usersToRestore) > 0 {
		fmt.Printf("[Gying] 发现 %d 个需要恢复的用户（使用加密密码重新登录）\n", len(usersToRestore))
		for i, user := range usersToRestore {
			if DebugLog {
				fmt.Printf("[Gying] [恢复用户 %d/%d] 处理: %s\n", i+1, len(usersToRestore), user.UsernameMasked)
			}
			
			// 解密密码
			password, err := p.decryptPassword(user.EncryptedPassword)
			if err != nil {
				fmt.Printf("[Gying] ❌ 用户 %s 解密密码失败: %v\n", user.UsernameMasked, err)
				continue
			}
			
			p.initOrRestoreUser(user.Username, password, "restore")
		}
	}

	// fmt.Printf("[Gying] ========== 所有账户初始化完成 ==========\n")
}

// initOrRestoreUser 初始化或恢复单个用户（登录并保存）
func (p *GyingPlugin) initOrRestoreUser(username, password, source string) {
	hash := p.generateHash(username)
	
	// 检查scraper是否已存在
	_, scraperExists := p.scrapers.Load(hash)
	if scraperExists {
		if DebugLog {
			fmt.Printf("[Gying] 用户 %s scraper已存在，跳过\n", p.maskUsername(username))
		}
		return
	}
	
	// 登录
	if DebugLog {
		fmt.Printf("[Gying] 开始登录账户: %s\n", username)
	}
	scraper, cookie, err := p.doLogin(username, password)
	if err != nil {
		fmt.Printf("[Gying] ❌ 账户 %s 登录失败: %v\n", username, err)
		return
	}

	if DebugLog {
		fmt.Printf("[Gying] 登录成功，已获取cloudscraper实例\n")
	}

	// 加密密码
	encryptedPassword, err := p.encryptPassword(password)
	if err != nil {
		fmt.Printf("[Gying] ❌ 加密密码失败: %v\n", err)
		return
	}
	
	// 保存用户
	user := &User{
		Hash:              hash,
		Username:          username,
		UsernameMasked:    p.maskUsername(username),
		EncryptedPassword: encryptedPassword,
		Cookie:            cookie,
		Status:            "active",
		CreatedAt:         time.Now(),
		LoginAt:           time.Now(),
		ExpireAt:          time.Now().AddDate(0, 4, 0), // 121天有效期
		LastAccessAt:      time.Now(),
	}
	
	// 保存scraper实例到内存
	p.scrapers.Store(hash, scraper)
	
	if err := p.saveUser(user); err != nil {
		fmt.Printf("[Gying] ❌ 保存账户失败: %v\n", err)
		return
	}

	fmt.Printf("[Gying] ✅ 账户 %s 初始化成功 (来源:%s)\n", user.UsernameMasked, source)
}

// getUserByHash 获取用户
func (p *GyingPlugin) getUserByHash(hash string) (*User, bool) {
	value, ok := p.users.Load(hash)
	if !ok {
		return nil, false
	}
	return value.(*User), true
}

// saveUser 保存用户
func (p *GyingPlugin) saveUser(user *User) error {
	p.users.Store(user.Hash, user)
	return p.persistUser(user)
}

// persistUser 持久化用户到文件
func (p *GyingPlugin) persistUser(user *User) error {
	filePath := filepath.Join(StorageDir, user.Hash+".json")
	data, err := json.MarshalIndent(user, "", "  ")
	if err != nil {
		return err
	}
	return ioutil.WriteFile(filePath, data, 0644)
}

// deleteUser 删除用户
func (p *GyingPlugin) deleteUser(hash string) error {
	p.users.Delete(hash)
	filePath := filepath.Join(StorageDir, hash+".json")
	return os.Remove(filePath)
}

// getActiveUsers 获取有效用户
func (p *GyingPlugin) getActiveUsers() []*User {
	var users []*User
	
	p.users.Range(func(key, value interface{}) bool {
		user := value.(*User)
		if user.Status == "active" && user.Cookie != "" {
			users = append(users, user)
		}
		return true
	})
	
	return users
}

// ============ HTTP路由处理 ============

// handleManagePage GET路由处理
func (p *GyingPlugin) handleManagePage(c *gin.Context) {
	param := c.Param("param")

	// 判断是用户名还是hash
	if len(param) == 64 && p.isHexString(param) {
		html := strings.ReplaceAll(HTMLTemplate, "HASH_PLACEHOLDER", param)
		c.Data(200, "text/html; charset=utf-8", []byte(html))
	} else {
		hash := p.generateHash(param)
		c.Redirect(302, "/gying/"+hash)
	}
}

// handleManagePagePOST POST路由处理
func (p *GyingPlugin) handleManagePagePOST(c *gin.Context) {
	hash := c.Param("param")

	var reqData map[string]interface{}
	if err := c.ShouldBindJSON(&reqData); err != nil {
		respondError(c, "无效的请求格式: "+err.Error())
		return
	}

	action, ok := reqData["action"].(string)
	if !ok || action == "" {
		respondError(c, "缺少action字段")
		return
	}

	switch action {
	case "get_status":
		p.handleGetStatus(c, hash)
	case "login":
		p.handleLogin(c, hash, reqData)
	case "logout":
		p.handleLogout(c, hash)
	case "test_search":
		p.handleTestSearch(c, hash, reqData)
	default:
		respondError(c, "未知的操作类型: "+action)
	}
}

// handleGetStatus 获取状态
func (p *GyingPlugin) handleGetStatus(c *gin.Context, hash string) {
	user, exists := p.getUserByHash(hash)

	if !exists {
		user = &User{
			Hash:         hash,
			Status:       "pending",
			CreatedAt:    time.Now(),
			LastAccessAt: time.Now(),
		}
		p.saveUser(user)
	} else {
		user.LastAccessAt = time.Now()
		p.saveUser(user)
	}

	loggedIn := false
	if user.Status == "active" && user.Cookie != "" {
		loggedIn = true
	}

	expiresInDays := 0
	if !user.ExpireAt.IsZero() {
		expiresInDays = int(time.Until(user.ExpireAt).Hours() / 24)
		if expiresInDays < 0 {
			expiresInDays = 0
		}
	}

	respondSuccess(c, "获取成功", gin.H{
		"hash":             hash,
		"logged_in":        loggedIn,
		"status":           user.Status,
		"username_masked":  user.UsernameMasked,
		"login_time":       user.LoginAt.Format("2006-01-02 15:04:05"),
		"expire_time":      user.ExpireAt.Format("2006-01-02 15:04:05"),
		"expires_in_days":  expiresInDays,
	})
}

// handleLogin 处理登录
func (p *GyingPlugin) handleLogin(c *gin.Context, hash string, reqData map[string]interface{}) {
	username, _ := reqData["username"].(string)
	password, _ := reqData["password"].(string)

	if username == "" || password == "" {
		respondError(c, "缺少用户名或密码")
		return
	}

	// 执行登录
	scraper, cookie, err := p.doLogin(username, password)
	if err != nil {
		respondError(c, "登录失败: "+err.Error())
		return
	}

	// 保存scraper实例到内存
	p.scrapers.Store(hash, scraper)

	// 加密密码
	encryptedPassword, err := p.encryptPassword(password)
	if err != nil {
		respondError(c, "加密密码失败: "+err.Error())
		return
	}
	
	// 保存用户
	user := &User{
		Hash:              hash,
		Username:          username,
		UsernameMasked:    p.maskUsername(username),
		EncryptedPassword: encryptedPassword,
		Cookie:            cookie,
		Status:            "active",
		LoginAt:           time.Now(),
		ExpireAt:          time.Now().AddDate(0, 4, 0), // 121天
		LastAccessAt:      time.Now(),
	}
	
	if _, exists := p.getUserByHash(hash); !exists {
		user.CreatedAt = time.Now()
	}

	if err := p.saveUser(user); err != nil {
		respondError(c, "保存失败: "+err.Error())
		return
	}

	respondSuccess(c, "登录成功", gin.H{
		"status":          "active",
		"username_masked": user.UsernameMasked,
	})
}

// handleLogout 退出登录
func (p *GyingPlugin) handleLogout(c *gin.Context, hash string) {
	user, exists := p.getUserByHash(hash)
	if !exists {
		respondError(c, "用户不存在")
		return
	}

	user.Cookie = ""
	user.Status = "pending"

	if err := p.saveUser(user); err != nil {
		respondError(c, "退出失败")
		return
	}

	respondSuccess(c, "已退出登录", gin.H{
		"status": "pending",
	})
}

// handleTestSearch 测试搜索
func (p *GyingPlugin) handleTestSearch(c *gin.Context, hash string, reqData map[string]interface{}) {
	keyword, ok := reqData["keyword"].(string)
	if !ok || keyword == "" {
		respondError(c, "缺少keyword字段")
		return
	}

	user, exists := p.getUserByHash(hash)
	if !exists || user.Cookie == "" {
		respondError(c, "请先登录")
		return
	}

	// 获取scraper实例
	scraperVal, exists := p.scrapers.Load(hash)
	if !exists {
		respondError(c, "用户scraper实例不存在，请重新登录")
		return
	}
	
	scraper, ok := scraperVal.(*cloudscraper.Scraper)
	if !ok || scraper == nil {
		respondError(c, "scraper实例无效，请重新登录")
		return
	}
	
	// 执行搜索（带403自动重新登录）
	results, err := p.searchWithScraperWithRetry(keyword, scraper, user)
	if err != nil {
		respondError(c, "搜索失败: "+err.Error())
		return
	}

	// 限制返回数量
	maxResults := 10
	if len(results) > maxResults {
		results = results[:maxResults]
	}

	// 转换为前端格式
	frontendResults := make([]gin.H, 0, len(results))
	for _, r := range results {
		links := make([]gin.H, 0, len(r.Links))
		for _, link := range r.Links {
			links = append(links, gin.H{
				"type":     link.Type,
				"url":      link.URL,
				"password": link.Password,
			})
		}

		frontendResults = append(frontendResults, gin.H{
			"title": r.Title,
			"links": links,
		})
	}

	respondSuccess(c, fmt.Sprintf("找到 %d 条结果", len(frontendResults)), gin.H{
		"keyword":       keyword,
		"total_results": len(frontendResults),
		"results":       frontendResults,
	})
}

// ============ 密码加密/解密 ============

// encryptPassword 使用AES加密密码
func (p *GyingPlugin) encryptPassword(password string) (string, error) {
	// 使用固定密钥（实际应用中可以使用配置或环境变量）
	key := []byte("gying-secret-key-32bytes-long!!!") // 32字节密钥用于AES-256
	
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	
	// 创建GCM模式
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	
	// 生成随机nonce
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	
	// 加密
	ciphertext := gcm.Seal(nonce, nonce, []byte(password), nil)
	
	// 返回base64编码的密文
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// decryptPassword 解密密码
func (p *GyingPlugin) decryptPassword(encrypted string) (string, error) {
	// 使用与加密相同的密钥
	key := []byte("gying-secret-key-32bytes-long!!!")
	
	// base64解码
	ciphertext, err := base64.StdEncoding.DecodeString(encrypted)
	if err != nil {
		return "", err
	}
	
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	
	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return "", fmt.Errorf("ciphertext too short")
	}
	
	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", err
	}
	
	return string(plaintext), nil
}

// ============ Cookie管理 ============

// createScraperWithCookies 创建一个带有指定cookies的cloudscraper实例
// 使用反射访问内部的http.Client并设置cookies到cookiejar
// 关键：禁用session refresh以防止cookies被清空
func (p *GyingPlugin) createScraperWithCookies(cookieStr string) (*cloudscraper.Scraper, error) {
	// 创建cloudscraper实例，配置以保护cookies不被刷新
	scraper, err := cloudscraper.New(
		cloudscraper.WithSessionConfig(
			false,              // refreshOn403 = false，禁用403时自动刷新
			365*24*time.Hour,   // interval = 1年，基本不刷新
			0,                  // maxRetries = 0
		),
	)
	if err != nil {
		return nil, fmt.Errorf("创建cloudscraper失败: %w", err)
	}
	
	// 如果有保存的cookies，使用反射设置到scraper的内部http.Client
	if cookieStr != "" {
		cookies := parseCookieString(cookieStr)
		
		if DebugLog {
			fmt.Printf("[Gying] 正在恢复 %d 个cookie到scraper实例\n", len(cookies))
		}
		
		// 使用反射访问scraper的unexported client字段
		scraperValue := reflect.ValueOf(scraper).Elem()
		clientField := scraperValue.FieldByName("client")
		
		if clientField.IsValid() && !clientField.IsNil() {
			// 使用反射访问client (需要使用Elem()因为是指针)
			clientValue := reflect.NewAt(clientField.Type(), unsafe.Pointer(clientField.UnsafeAddr())).Elem()
			client, ok := clientValue.Interface().(*http.Client)
			
			if ok && client != nil && client.Jar != nil {
				// 将cookies设置到cookiejar
				// 注意：必须使用正确的URL和cookie属性
				gyingURL, _ := url.Parse("https://www.gying.net")
				var httpCookies []*http.Cookie
				
				for name, value := range cookies {
					cookie := &http.Cookie{
						Name:   name,
						Value:  value,
						// 不设置Domain和Path，让cookiejar根据URL自动推导
						// cookiejar.SetCookies会根据提供的URL自动设置正确的Domain和Path
					}
					httpCookies = append(httpCookies, cookie)
					
					if DebugLog {
						fmt.Printf("[Gying]   准备恢复Cookie: %s=%s\n", 
							cookie.Name, cookie.Value[:min(10, len(cookie.Value))])
					}
				}
				
				client.Jar.SetCookies(gyingURL, httpCookies)
				
				// 验证cookies是否被正确设置
				if DebugLog {
					storedCookies := client.Jar.Cookies(gyingURL)
					fmt.Printf("[Gying] ✅ 成功恢复 %d 个cookie到scraper的cookiejar\n", len(cookies))
					fmt.Printf("[Gying] 验证: cookiejar中现有 %d 个cookie\n", len(storedCookies))
					
					// 详细打印每个cookie以便调试  
					for i, c := range storedCookies {
						fmt.Printf("[Gying]   设置后Cookie[%d]: %s=%s (Domain:%s, Path:%s)\n", 
							i, c.Name, c.Value[:min(10, len(c.Value))], c.Domain, c.Path)
					}
				}
			} else {
				if DebugLog {
					fmt.Printf("[Gying] ⚠️  无法获取http.Client或其Jar\n")
				}
			}
		} else {
			if DebugLog {
				fmt.Printf("[Gying] ⚠️  无法通过反射访问client字段\n")
			}
		}
	}
	
	return scraper, nil
}

// parseCookieString 解析cookie字符串为map
func parseCookieString(cookieStr string) map[string]string {
	cookies := make(map[string]string)
	parts := strings.Split(cookieStr, ";")
	
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if idx := strings.Index(part, "="); idx > 0 {
			name := part[:idx]
			value := part[idx+1:]
			cookies[name] = value
		}
	}
	
	return cookies
}

// ============ 登录逻辑 ============

// doLogin 执行登录，返回scraper实例和cookie字符串
// 
// 登录流程（3步）：
//   1. GET登录页 (https://www.gying.net/user/login/) → 获取PHPSESSID
//   2. POST登录  (https://www.gying.net/user/login)  → 获取BT_auth、BT_cookietime等认证cookies
//   3. GET详情页 (https://www.gying.net/mv/wkMn)     → 触发防爬cookies (vrg_sc、vrg_go等)
//
// 返回: (*cloudscraper.Scraper, cookie字符串, error)
func (p *GyingPlugin) doLogin(username, password string) (*cloudscraper.Scraper, string, error) {
	if DebugLog {
		fmt.Printf("[Gying] ========== 开始登录 ==========\n")
		fmt.Printf("[Gying] 用户名: %s\n", username)
		fmt.Printf("[Gying] 密码长度: %d\n", len(password))
	}

	// 创建cloudscraper实例（每个用户独立的实例）
	// 关键配置：禁用403自动刷新,防止cookie被清空
	scraper, err := cloudscraper.New(
		cloudscraper.WithSessionConfig(
			false,              // refreshOn403 = false，禁用403时自动刷新（重要！）
			365*24*time.Hour,   // interval = 1年，基本不刷新
			0,                  // maxRetries = 0
		),
	)
	if err != nil {
		if DebugLog {
			fmt.Printf("[Gying] 创建cloudscraper失败: %v\n", err)
		}
		return nil, "", fmt.Errorf("创建cloudscraper失败: %w", err)
	}

	if DebugLog {
		fmt.Printf("[Gying] cloudscraper创建成功（已禁用403自动刷新）\n")
	}

	// 创建cookieMap用于收集所有cookies
	cookieMap := make(map[string]string)
	
	// ========== 步骤1: GET登录页 (获取初始PHPSESSID) ==========
	loginPageURL := "https://www.gying.net/user/login/"
	if DebugLog {
		fmt.Printf("[Gying] 步骤1: 访问登录页面: %s\n", loginPageURL)
	}

	getResp, err := scraper.Get(loginPageURL)
	if err != nil {
		if DebugLog {
			fmt.Printf("[Gying] 访问登录页面失败: %v\n", err)
		}
		return nil, "", fmt.Errorf("访问登录页面失败: %w", err)
	}
	defer getResp.Body.Close()
	ioutil.ReadAll(getResp.Body) // 读取body

	if DebugLog {
		fmt.Printf("[Gying] 登录页面状态码: %d\n", getResp.StatusCode)
	}
	
	// 从登录页响应中收集cookies
	for _, setCookie := range getResp.Header["Set-Cookie"] {
		parts := strings.Split(setCookie, ";")
		if len(parts) > 0 {
			cookiePart := strings.TrimSpace(parts[0])
			if idx := strings.Index(cookiePart, "="); idx > 0 {
				name := cookiePart[:idx]
				value := cookiePart[idx+1:]
				cookieMap[name] = value
				if DebugLog {
					displayValue := value
					if len(displayValue) > 20 {
						displayValue = displayValue[:20] + "..."
					}
					fmt.Printf("[Gying]   登录页Cookie: %s=%s\n", name, displayValue)
				}
			}
		}
	}

	// ========== 步骤2: POST登录 (获取认证cookies) ==========
	loginURL := "https://www.gying.net/user/login"
	postData := fmt.Sprintf("code=&siteid=1&dosubmit=1&cookietime=10506240&username=%s&password=%s",
		url.QueryEscape(username),
		url.QueryEscape(password))

	if DebugLog {
		fmt.Printf("[Gying] 步骤2: POST登录\n")
		fmt.Printf("[Gying] 登录URL: %s\n", loginURL)
		fmt.Printf("[Gying] POST数据: %s\n", postData)
	}

	resp, err := scraper.Post(loginURL, "application/x-www-form-urlencoded", strings.NewReader(postData))
	if err != nil {
		if DebugLog {
			fmt.Printf("[Gying] 登录POST请求失败: %v\n", err)
		}
		return nil, "", fmt.Errorf("登录POST请求失败: %w", err)
	}
	defer resp.Body.Close()

	if DebugLog {
		fmt.Printf("[Gying] 响应状态码: %d\n", resp.StatusCode)
	}
	
	// 从POST登录响应中收集cookies
	for _, setCookie := range resp.Header["Set-Cookie"] {
		parts := strings.Split(setCookie, ";")
		if len(parts) > 0 {
			cookiePart := strings.TrimSpace(parts[0])
			if idx := strings.Index(cookiePart, "="); idx > 0 {
				name := cookiePart[:idx]
				value := cookiePart[idx+1:]
				cookieMap[name] = value
				if DebugLog {
					displayValue := value
					if len(displayValue) > 20 {
						displayValue = displayValue[:20] + "..."
					}
					fmt.Printf("[Gying]   POST登录Cookie: %s=%s\n", name, displayValue)
				}
			}
		}
	}

	// 读取响应
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		if DebugLog {
			fmt.Printf("[Gying] 读取响应失败: %v\n", err)
		}
		return nil, "", fmt.Errorf("读取响应失败: %w", err)
	}

	if DebugLog {
		fmt.Printf("[Gying] 响应内容: %s\n", string(body))
	}

	var loginResp map[string]interface{}
	if err := json.Unmarshal(body, &loginResp); err != nil {
		if DebugLog {
			fmt.Printf("[Gying] JSON解析失败: %v\n", err)
		}
		return nil, "", fmt.Errorf("JSON解析失败: %w, 响应内容: %s", err, string(body))
	}

	if DebugLog {
		fmt.Printf("[Gying] 解析后的响应: %+v\n", loginResp)
		fmt.Printf("[Gying] code字段类型: %T, 值: %v\n", loginResp["code"], loginResp["code"])
	}

	// 检查登录结果（兼容多种类型：int、float64、json.Number、string）
	var codeValue int
	codeInterface := loginResp["code"]
	
	switch v := codeInterface.(type) {
	case int:
		codeValue = v
	case float64:
		codeValue = int(v)
	case int64:
		codeValue = int(v)
	default:
		// 尝试转换为字符串再解析
		codeStr := fmt.Sprintf("%v", codeInterface)
		parsed, err := strconv.Atoi(codeStr)
		if err != nil {
			if DebugLog {
				fmt.Printf("[Gying] 无法解析code字段: %T, 值: %v, 错误: %v\n", codeInterface, codeInterface, err)
			}
			return nil, "", fmt.Errorf("无法解析code字段，类型: %T, 值: %v", codeInterface, codeInterface)
		}
		codeValue = parsed
	}

	if DebugLog {
		fmt.Printf("[Gying] 解析后的code值: %d\n", codeValue)
	}

	if codeValue != 200 {
		if DebugLog {
			fmt.Printf("[Gying] 登录失败: code=%d (期望200)\n", codeValue)
		}
		return nil, "", fmt.Errorf("登录失败: code=%d, 响应=%s", codeValue, string(body))
	}

	// ========== 步骤3: GET详情页 (触发防爬cookies如vrg_sc、vrg_go等) ==========
	if DebugLog {
		fmt.Printf("[Gying] 步骤3: GET详情页收集完整Cookie\n")
	}
	
	detailResp, err := scraper.Get("https://www.gying.net/mv/wkMn")
	if err == nil {
		defer detailResp.Body.Close()
		ioutil.ReadAll(detailResp.Body)
		
		if DebugLog {
			fmt.Printf("[Gying] 详情页状态码: %d\n", detailResp.StatusCode)
		}
		
		// 从详情页响应中收集cookies
		for _, setCookie := range detailResp.Header["Set-Cookie"] {
			parts := strings.Split(setCookie, ";")
			if len(parts) > 0 {
				cookiePart := strings.TrimSpace(parts[0])
				if idx := strings.Index(cookiePart, "="); idx > 0 {
					name := cookiePart[:idx]
					value := cookiePart[idx+1:]
					cookieMap[name] = value
					if DebugLog {
						displayValue := value
						if len(displayValue) > 30 {
							displayValue = displayValue[:30] + "..."
						}
						fmt.Printf("[Gying]   详情页Cookie: %s=%s\n", name, displayValue)
					}
				}
			}
		}
	}
	
	// 构建cookie字符串
	var cookieParts []string
	for name, value := range cookieMap {
		cookieParts = append(cookieParts, fmt.Sprintf("%s=%s", name, value))
	}
	cookieStr := strings.Join(cookieParts, "; ")
	
	if DebugLog {
		fmt.Printf("[Gying] ✅ 登录成功！提取到 %d 个Cookie\n", len(cookieMap))
		fmt.Printf("[Gying] Cookie字符串长度: %d\n", len(cookieStr))
		for name, value := range cookieMap {
			displayValue := value
			if len(displayValue) > 30 {
				displayValue = displayValue[:30] + "..."
			}
			fmt.Printf("[Gying]   %s=%s (len:%d)\n", name, displayValue, len(value))
		}
		fmt.Printf("[Gying] ========== 登录完成 ==========\n")
	}

	// 返回scraper实例和实际的cookie字符串
	return scraper, cookieStr, nil
}

// min 辅助函数
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// ============ 重新登录逻辑 ============

// reloginUser 重新登录指定用户
func (p *GyingPlugin) reloginUser(user *User) error {
	if DebugLog {
		fmt.Printf("[Gying] 🔄 开始重新登录用户: %s\n", user.UsernameMasked)
	}
	
	// 解密密码
	password, err := p.decryptPassword(user.EncryptedPassword)
	if err != nil {
		if DebugLog {
			fmt.Printf("[Gying] ❌ 解密密码失败: %v\n", err)
		}
		return fmt.Errorf("解密密码失败: %w", err)
	}
	
	// 执行登录
	scraper, cookie, err := p.doLogin(user.Username, password)
	if err != nil {
		if DebugLog {
			fmt.Printf("[Gying] ❌ 重新登录失败: %v\n", err)
		}
		return fmt.Errorf("重新登录失败: %w", err)
	}
	
	// 更新scraper实例
	p.scrapers.Store(user.Hash, scraper)
	
	// 更新用户信息
	user.Cookie = cookie
	user.LoginAt = time.Now()
	user.ExpireAt = time.Now().AddDate(0, 4, 0)
	user.Status = "active"
	
	if err := p.saveUser(user); err != nil {
		if DebugLog {
			fmt.Printf("[Gying] ⚠️  保存用户失败: %v\n", err)
		}
	}
	
	if DebugLog {
		fmt.Printf("[Gying] ✅ 用户 %s 重新登录成功\n", user.UsernameMasked)
	}
	
	return nil
}

// ============ 搜索逻辑 ============

// executeSearchTasks 并发执行搜索任务
func (p *GyingPlugin) executeSearchTasks(users []*User, keyword string) []model.SearchResult {
	var allResults []model.SearchResult
	var mu sync.Mutex
	var wg sync.WaitGroup

	for _, user := range users {
		wg.Add(1)
		go func(u *User) {
			defer wg.Done()

			// 获取用户的scraper实例
			scraperVal, exists := p.scrapers.Load(u.Hash)
			var scraper *cloudscraper.Scraper
			
			if !exists {
				if DebugLog {
					fmt.Printf("[Gying] 用户 %s 没有scraper实例，尝试使用已保存的cookie创建\n", u.UsernameMasked)
				}
				
				// 使用已保存的cookie创建scraper实例（关键！）
				newScraper, err := p.createScraperWithCookies(u.Cookie)
				if err != nil {
					if DebugLog {
						fmt.Printf("[Gying] 为用户 %s 创建scraper失败: %v\n", u.UsernameMasked, err)
					}
					return
				}
				
				// 存储新创建的scraper实例
				p.scrapers.Store(u.Hash, newScraper)
				scraper = newScraper
				
				if DebugLog {
					fmt.Printf("[Gying] 已为用户 %s 恢复scraper实例（含cookie）\n", u.UsernameMasked)
				}
			} else {
				var ok bool
				scraper, ok = scraperVal.(*cloudscraper.Scraper)
				if !ok || scraper == nil {
					if DebugLog {
						fmt.Printf("[Gying] 用户 %s scraper实例无效，跳过\n", u.UsernameMasked)
					}
					return
				}
			}

			results, err := p.searchWithScraperWithRetry(keyword, scraper, u)
			if err != nil {
				if DebugLog {
					fmt.Printf("[Gying] 用户 %s 搜索失败（已重试）: %v\n", u.UsernameMasked, err)
				}
				return
			}

			mu.Lock()
			allResults = append(allResults, results...)
			mu.Unlock()
		}(user)
	}

	wg.Wait()

	// 去重
	return p.deduplicateResults(allResults)
}

// searchWithScraperWithRetry 使用scraper搜索（带403自动重新登录重试）
func (p *GyingPlugin) searchWithScraperWithRetry(keyword string, scraper *cloudscraper.Scraper, user *User) ([]model.SearchResult, error) {
	results, err := p.searchWithScraper(keyword, scraper)
	
	// 检测是否为403错误
	if err != nil && strings.Contains(err.Error(), "403") {
		if DebugLog {
			fmt.Printf("[Gying] ⚠️  检测到403错误，尝试重新登录用户 %s\n", user.UsernameMasked)
		}
		
		// 尝试重新登录
		if reloginErr := p.reloginUser(user); reloginErr != nil {
			if DebugLog {
				fmt.Printf("[Gying] ❌ 重新登录失败: %v\n", reloginErr)
			}
			return nil, fmt.Errorf("403错误且重新登录失败: %w", reloginErr)
		}
		
		// 获取新的scraper实例
		scraperVal, exists := p.scrapers.Load(user.Hash)
		if !exists {
			return nil, fmt.Errorf("重新登录后未找到scraper实例")
		}
		
		newScraper, ok := scraperVal.(*cloudscraper.Scraper)
		if !ok || newScraper == nil {
			return nil, fmt.Errorf("重新登录后scraper实例无效")
		}
		
		// 使用新scraper重试搜索
		if DebugLog {
			fmt.Printf("[Gying] 🔄 使用新登录状态重试搜索\n")
		}
		results, err = p.searchWithScraper(keyword, newScraper)
		if err != nil {
			return nil, fmt.Errorf("重新登录后搜索仍然失败: %w", err)
		}
	}
	
	return results, err
}

// searchWithScraper 使用scraper搜索
func (p *GyingPlugin) searchWithScraper(keyword string, scraper *cloudscraper.Scraper) ([]model.SearchResult, error) {
	if DebugLog {
		fmt.Printf("[Gying] ---------- searchWithScraper 开始 ----------\n")
		fmt.Printf("[Gying] 关键词: %s\n", keyword)
	}

	// 1. 使用cloudscraper请求搜索页面
	searchURL := fmt.Sprintf("https://www.gying.net/s/2-0--1/%s", url.QueryEscape(keyword))
	
	if DebugLog {
		fmt.Printf("[Gying] %s\n", "www.gying.fun")
		fmt.Printf("[Gying] 使用cloudscraper发送请求\n")
	}

	resp, err := scraper.Get(searchURL)
	if err != nil {
		if DebugLog {
			fmt.Printf("[Gying] 搜索请求失败: %v\n", err)
		}
		return nil, err
	}
	defer resp.Body.Close()

	if DebugLog {
		fmt.Printf("[Gying] 搜索响应状态码: %d\n", resp.StatusCode)
	}
	
	// 读取响应body
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		if DebugLog {
			fmt.Printf("[Gying] 读取响应失败: %v\n", err)
		}
		return nil, err
	}

	if DebugLog {
		fmt.Printf("[Gying] 响应Body长度: %d 字节\n", len(body))
		if len(body) > 0 {
			// 打印前500字符
			preview := string(body)
			if len(preview) > 500 {
				preview = preview[:500] + "..."
			}
			fmt.Printf("[Gying] 响应预览: %s\n", preview)
		}
	}
	
	// 检查403错误
	if resp.StatusCode == 403 {
		if DebugLog {
			fmt.Printf("[Gying] ❌ 收到403 Forbidden - Cookie可能已过期或被网站拒绝\n")
			if len(body) > 0 {
				preview := string(body)
				if len(preview) > 300 {
					preview = preview[:300] + "..."
				}
				fmt.Printf("[Gying] 403响应内容: %s\n", preview)
			}
		}
		return nil, fmt.Errorf("HTTP 403 Forbidden - 可能需要重新登录")
	}

	// 2. 提取 _obj.search JSON
	re := regexp.MustCompile(`_obj\.search=(\{.*?\});`)
	matches := re.FindSubmatch(body)
	
	if DebugLog {
		fmt.Printf("[Gying] 正则匹配结果: 找到 %d 个匹配\n", len(matches))
	}

	if len(matches) < 2 {
		if DebugLog {
			fmt.Printf("[Gying] ❌ 未找到 _obj.search JSON数据\n")
			// 尝试查找是否有其他模式
			if strings.Contains(string(body), "_obj.search") {
				fmt.Printf("[Gying] 但是Body中包含 '_obj.search' 字符串\n")
			} else {
				fmt.Printf("[Gying] Body中不包含 '_obj.search' 字符串\n")
			}
		}
		return nil, fmt.Errorf("未找到搜索结果数据")
	}

	if DebugLog {
		jsonStr := string(matches[1])
		if len(jsonStr) > 200 {
			jsonStr = jsonStr[:200] + "..."
		}
		fmt.Printf("[Gying] 提取的JSON数据: %s\n", jsonStr)
	}

	var searchData SearchData
	if err := json.Unmarshal(matches[1], &searchData); err != nil {
		if DebugLog {
			fmt.Printf("[Gying] JSON解析失败: %v\n", err)
			fmt.Printf("[Gying] 原始JSON: %s\n", string(matches[1]))
		}
		return nil, fmt.Errorf("解析搜索数据失败: %w", err)
	}

	if DebugLog {
		fmt.Printf("[Gying] 搜索数据解析成功:\n")
		fmt.Printf("[Gying]   - 关键词: %s\n", searchData.Q)
		fmt.Printf("[Gying]   - 结果数量字符串: %s\n", searchData.N)
		fmt.Printf("[Gying]   - 资源ID数组长度: %d\n", len(searchData.L.I))
		fmt.Printf("[Gying]   - 标题数组长度: %d\n", len(searchData.L.Title))
		if len(searchData.L.I) > 0 {
			fmt.Printf("[Gying]   - 前3个资源ID: %v\n", searchData.L.I[:min(3, len(searchData.L.I))])
			fmt.Printf("[Gying]   - 前3个标题: %v\n", searchData.L.Title[:min(3, len(searchData.L.Title))])
		}
	}

	// 3. 刷新防爬cookies（关键！访问详情页触发vrg_sc、vrg_go等防爬cookies）
	if DebugLog {
		fmt.Printf("[Gying] 刷新防爬cookies...\n")
	}
	refreshResp, err := scraper.Get("https://www.gying.net/mv/wkMn")
	if err == nil && refreshResp != nil {
		refreshResp.Body.Close()
		if DebugLog {
			fmt.Printf("[Gying] 防爬cookies刷新成功 (状态码: %d)\n", refreshResp.StatusCode)
		}
	}
	
	// 4. 并发请求详情接口
	results, err := p.fetchAllDetails(&searchData, scraper, keyword)
	if err != nil {
		if DebugLog {
			fmt.Printf("[Gying] fetchAllDetails 失败: %v\n", err)
			fmt.Printf("[Gying] ---------- searchWithScraper 结束 ----------\n")
		}
		return nil, err
	}
	
	if DebugLog {
		fmt.Printf("[Gying] fetchAllDetails 返回 %d 条结果\n", len(results))
		fmt.Printf("[Gying] ---------- searchWithScraper 结束 ----------\n")
	}

	return results, nil
}

// fetchAllDetails 并发获取所有详情
func (p *GyingPlugin) fetchAllDetails(searchData *SearchData, scraper *cloudscraper.Scraper, keyword string) ([]model.SearchResult, error) {
	if DebugLog {
		fmt.Printf("[Gying] >>> fetchAllDetails 开始\n")
		fmt.Printf("[Gying] 需要获取 %d 个详情，关键词: %s\n", len(searchData.L.I), keyword)
	}

	var results []model.SearchResult
	var mu sync.Mutex
	var wg sync.WaitGroup

	semaphore := make(chan struct{}, MaxConcurrentDetails)
	errChan := make(chan error, 1) // 用于接收403错误

	successCount := 0
	failCount := 0
	has403 := false
	
	// 将关键词转为小写，用于不区分大小写的匹配
	keywordLower := strings.ToLower(keyword)

	for i := 0; i < len(searchData.L.I); i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()

			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			// 检查是否已经遇到403错误
			mu.Lock()
			if has403 {
				mu.Unlock()
				return
			}
			mu.Unlock()

			// 检查标题是否包含搜索关键词
			if index >= len(searchData.L.Title) {
				if DebugLog {
					fmt.Printf("[Gying]   [%d/%d] ⏭️  跳过: 索引超出标题数组范围\n", 
						index+1, len(searchData.L.I))
				}
				return
			}
			
			title := searchData.L.Title[index]
			titleLower := strings.ToLower(title)
			if !strings.Contains(titleLower, keywordLower) {
				if DebugLog {
					fmt.Printf("[Gying]   [%d/%d] ⏭️  跳过: 标题不包含关键词 '%s' (标题: %s)\n", 
						index+1, len(searchData.L.I), keyword, title)
				}
				return
			}

			if DebugLog {
				fmt.Printf("[Gying]   [%d/%d] 获取详情: ID=%s, Type=%s, 标题=%s\n", 
					index+1, len(searchData.L.I), searchData.L.I[index], searchData.L.D[index], title)
			}

			detail, err := p.fetchDetail(searchData.L.I[index], searchData.L.D[index], scraper)
			if err != nil {
				if DebugLog {
					fmt.Printf("[Gying]   [%d/%d] ❌ 获取详情失败: %v\n", index+1, len(searchData.L.I), err)
				}
				
				// 检查是否是403错误
				if strings.Contains(err.Error(), "403") {
					mu.Lock()
					if !has403 {
						has403 = true
						select {
						case errChan <- err:
						default:
						}
					}
					mu.Unlock()
				}
				
				mu.Lock()
				failCount++
				mu.Unlock()
				return
			}

			result := p.buildResult(detail, searchData, index)
			if result.Title != "" && len(result.Links) > 0 {
				if DebugLog {
					fmt.Printf("[Gying]   [%d/%d] ✅ 成功: %s (%d个链接)\n", 
						index+1, len(searchData.L.I), result.Title, len(result.Links))
				}
				mu.Lock()
				results = append(results, result)
				successCount++
				mu.Unlock()
			} else {
				if DebugLog {
					fmt.Printf("[Gying]   [%d/%d] ⚠️  跳过: 标题或链接为空 (标题:%s, 链接数:%d)\n", 
						index+1, len(searchData.L.I), result.Title, len(result.Links))
				}
			}
		}(i)
	}

	wg.Wait()

	// 检查是否有403错误
	select {
	case err := <-errChan:
		if DebugLog {
			fmt.Printf("[Gying] <<< fetchAllDetails 检测到403错误，需要重新登录\n")
		}
		return nil, err
	default:
	}

	if DebugLog {
		fmt.Printf("[Gying] <<< fetchAllDetails 完成: 成功=%d, 失败=%d, 总计=%d\n", 
			successCount, failCount, len(searchData.L.I))
	}

	return results, nil
}

// fetchDetail 获取详情
func (p *GyingPlugin) fetchDetail(resourceID, resourceType string, scraper *cloudscraper.Scraper) (*DetailData, error) {
	detailURL := fmt.Sprintf("https://www.gying.net/res/downurl/%s/%s", resourceType, resourceID)
	
	if DebugLog {
		fmt.Printf("[Gying]     fetchDetail: %s\n", detailURL)
	}

	// 使用cloudscraper发送请求（自动管理Cookie和绕过反爬虫）
	resp, err := scraper.Get(detailURL)

	if err != nil {
		if DebugLog {
			fmt.Printf("[Gying]     请求失败: %v\n", err)
		}
		return nil, err
	}
	defer resp.Body.Close()

	if DebugLog {
		fmt.Printf("[Gying]     响应状态码: %d\n", resp.StatusCode)
	}

	// 检查403错误
	if resp.StatusCode == 403 {
		if DebugLog {
			fmt.Printf("[Gying]     ❌ 详情接口返回403 - Cookie可能已过期\n")
		}
		return nil, fmt.Errorf("HTTP 403 Forbidden")
	}

	if resp.StatusCode != 200 {
		if DebugLog {
			fmt.Printf("[Gying]     ❌ HTTP错误: %d\n", resp.StatusCode)
		}
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		if DebugLog {
			fmt.Printf("[Gying]     读取响应失败: %v\n", err)
		}
		return nil, err
	}

	if DebugLog {
		fmt.Printf("[Gying]     响应长度: %d 字节\n", len(body))
	}

	var detail DetailData
	if err := json.Unmarshal(body, &detail); err != nil {
		if DebugLog {
			fmt.Printf("[Gying]     JSON解析失败: %v\n", err)
			// 打印前200字符
			preview := string(body)
			if len(preview) > 200 {
				preview = preview[:200] + "..."
			}
			fmt.Printf("[Gying]     响应内容: %s\n", preview)
		}
		return nil, err
	}

	if DebugLog {
		fmt.Printf("[Gying]     详情Code: %d, 网盘链接数: %d\n", detail.Code, len(detail.Panlist.URL))
	}

	// 检查JSON响应中的code字段（关键！）
	if detail.Code == 403 {
		if DebugLog {
			fmt.Printf("[Gying]     ❌ 详情接口返回Code=403 - 登录状态可能已失效\n")
		}
		return nil, fmt.Errorf("Detail API returned code 403 - authentication may have expired")
	}

	return &detail, nil
}

// buildResult 构建SearchResult
func (p *GyingPlugin) buildResult(detail *DetailData, searchData *SearchData, index int) model.SearchResult {
	if index >= len(searchData.L.Title) {
		return model.SearchResult{}
	}

	title := searchData.L.Title[index]
	resourceType := searchData.L.D[index]
	resourceID := searchData.L.I[index]
	
	// 获取年份并拼接到标题后面
	var year int
	if index < len(searchData.L.Year) && searchData.L.Year[index] > 0 {
		year = searchData.L.Year[index]
		// 拼接年份到标题：遮天（2023）
		title = fmt.Sprintf("%s（%d）", title, year)
	}

	// 构建描述
	var contentParts []string
	if index < len(searchData.L.Info) && searchData.L.Info[index] != "" {
		contentParts = append(contentParts, searchData.L.Info[index])
	}
	if index < len(searchData.L.Daoyan) && searchData.L.Daoyan[index] != "" {
		contentParts = append(contentParts, fmt.Sprintf("导演: %s", searchData.L.Daoyan[index]))
	}
	if index < len(searchData.L.Zhuyan) && searchData.L.Zhuyan[index] != "" {
		contentParts = append(contentParts, fmt.Sprintf("主演: %s", searchData.L.Zhuyan[index]))
	}

	// 提取网盘链接
	links := p.extractPanLinks(detail)

	// 构建标签（保留年份标签，提供额外的过滤维度）
	var tags []string
	if year > 0 {
		tags = append(tags, fmt.Sprintf("%d", year))
	}

	// 从网盘时间数组中选择最新的时间（最小的相对时间值）
	// 检查 detail 是否为 nil
	var datetime time.Time
	if detail == nil {
		if DebugLog {
			fmt.Printf("[Gying] buildResult: detail为nil，使用当前时间\n")
		}
		datetime = time.Now()
	} else {
		datetime = p.parseUpdateTime(detail.Panlist.Time)
		if DebugLog {
			fmt.Printf("[Gying] buildResult时间解析: 时间数组长度=%d, 解析后时间=%v\n", 
				len(detail.Panlist.Time), datetime.Format("2006-01-02 15:04:05"))
			if len(detail.Panlist.Time) > 0 {
				fmt.Printf("[Gying]   前3个时间字符串: %v\n", detail.Panlist.Time[:min(3, len(detail.Panlist.Time))])
			}
		}
	}

	return model.SearchResult{
		UniqueID: fmt.Sprintf("gying-%s-%s", resourceType, resourceID),
		Title:    title,
		Content:  strings.Join(contentParts, " | "),
		Links:    links,
		Tags:     tags,
		Channel:  "", // 插件搜索结果Channel为空
		Datetime: datetime,
	}
}

// parseUpdateTime 解析网盘更新时间数组，返回最新的更新时间
// 时间字符串格式：["今天", "昨天", "2天前", "1月前", "1年前"] 等
func (p *GyingPlugin) parseUpdateTime(timeStrs []string) time.Time {
	// 处理 nil slice 的情况
	if timeStrs == nil || len(timeStrs) == 0 {
		if DebugLog {
			fmt.Printf("[Gying] parseUpdateTime: 时间数组为空或nil，返回当前时间\n")
		}
		// 如果没有时间信息，返回当前时间
		return time.Now()
	}

	now := time.Now()
	var latestTime *time.Time

	if DebugLog {
		fmt.Printf("[Gying] parseUpdateTime: 开始解析 %d 个时间字符串\n", len(timeStrs))
	}

	// 遍历所有时间字符串，找到最新的（最接近当前时间的）那个
	for i, timeStr := range timeStrs {
		if timeStr == "" {
			continue
		}

		parsedTime := p.parseRelativeTime(timeStr, now)
		if parsedTime != nil {
			if DebugLog && i < 5 { // 只打印前5个，避免日志过多
				fmt.Printf("[Gying]   [%d] '%s' -> %v\n", i, timeStr, parsedTime.Format("2006-01-02 15:04:05"))
			}
			// 找到最接近当前时间的（最新的）
			if latestTime == nil || parsedTime.After(*latestTime) {
				latestTime = parsedTime
			}
		} else {
			if DebugLog && i < 5 {
				fmt.Printf("[Gying]   [%d] '%s' -> 解析失败\n", i, timeStr)
			}
		}
	}

	// 如果解析失败，返回当前时间
	if latestTime == nil {
		if DebugLog {
			fmt.Printf("[Gying] parseUpdateTime: 所有时间解析失败，返回当前时间\n")
			// 输出前几个时间字符串用于调试
			if len(timeStrs) > 0 {
				fmt.Printf("[Gying]   前3个时间字符串: %v\n", timeStrs[:min(3, len(timeStrs))])
			}
		}
		return time.Now()
	}

	if DebugLog {
		fmt.Printf("[Gying] parseUpdateTime: 最终选择时间 %v\n", latestTime.Format("2006-01-02 15:04:05"))
	}
	return *latestTime
}

// parseRelativeTime 解析单个相对时间字符串，返回对应的time.Time
// 支持格式：今天、昨天、N天前、N月前、N年前
func (p *GyingPlugin) parseRelativeTime(timeStr string, baseTime time.Time) *time.Time {
	timeStr = strings.TrimSpace(timeStr)
	if timeStr == "" {
		return nil
	}

	switch timeStr {
	case "今天":
		t := baseTime.Truncate(24 * time.Hour)
		return &t
	case "昨天":
		t := baseTime.AddDate(0, 0, -1).Truncate(24 * time.Hour)
		return &t
	default:
		// 解析 "N天前"、"N月前"、"N年前" 格式
		if strings.HasSuffix(timeStr, "天前") {
			daysStr := strings.TrimSuffix(timeStr, "天前")
			days, err := strconv.Atoi(daysStr)
			if err == nil && days >= 0 {
				t := baseTime.AddDate(0, 0, -days).Truncate(24 * time.Hour)
				return &t
			}
		} else if strings.HasSuffix(timeStr, "月前") {
			monthsStr := strings.TrimSuffix(timeStr, "月前")
			months, err := strconv.Atoi(monthsStr)
			if err == nil && months >= 0 {
				t := baseTime.AddDate(0, -months, 0).Truncate(24 * time.Hour)
				return &t
			}
		} else if strings.HasSuffix(timeStr, "年前") {
			yearsStr := strings.TrimSuffix(timeStr, "年前")
			years, err := strconv.Atoi(yearsStr)
			if err == nil && years >= 0 {
				t := baseTime.AddDate(-years, 0, 0).Truncate(24 * time.Hour)
				return &t
			}
		}
	}

	// 无法解析，返回nil
	return nil
}

// extractPanLinks 提取网盘链接
func (p *GyingPlugin) extractPanLinks(detail *DetailData) []model.Link {
	var links []model.Link
	seen := make(map[string]bool)

	for i := 0; i < len(detail.Panlist.URL); i++ {
		linkURL := strings.TrimSpace(detail.Panlist.URL[i])
		
		// 去除URL中的访问码标记
		linkURL = regexp.MustCompile(`（访问码：.*?）`).ReplaceAllString(linkURL, "")
		linkURL = regexp.MustCompile(`\(访问码：.*?\)`).ReplaceAllString(linkURL, "")
		linkURL = strings.TrimSpace(linkURL)

		if linkURL == "" || seen[linkURL] {
			continue
		}
		seen[linkURL] = true

		// 识别网盘类型
		linkType := p.determineLinkType(linkURL)
		if linkType == "others" {
			continue
		}

		// 提取提取码
		password := ""
		if i < len(detail.Panlist.P) && detail.Panlist.P[i] != "" {
			password = detail.Panlist.P[i]
		}
		
		// 从URL提取提取码（优先）
		if urlPwd := p.extractPasswordFromURL(linkURL); urlPwd != "" {
			password = urlPwd
		}

		// 解析对应的时间
		var linkDatetime time.Time
		if i < len(detail.Panlist.Time) && detail.Panlist.Time[i] != "" {
			timeStr := detail.Panlist.Time[i]
			parsedTime := p.parseRelativeTime(timeStr, time.Now())
			if parsedTime != nil {
				linkDatetime = *parsedTime
			}
			// 如果解析失败，保持为零值，合并逻辑会使用result.Datetime
		}
		// 如果没有时间信息，保持为零值，合并逻辑会使用result.Datetime

		links = append(links, model.Link{
			Type:     linkType,
			URL:      linkURL,
			Password: password,
			Datetime: linkDatetime,
		})
	}

	return links
}

// determineLinkType 识别网盘类型
func (p *GyingPlugin) determineLinkType(linkURL string) string {
	switch {
	case strings.Contains(linkURL, "pan.quark.cn"):
		return "quark"
	case strings.Contains(linkURL, "drive.uc.cn"):
		return "uc"
	case strings.Contains(linkURL, "pan.baidu.com"):
		return "baidu"
	case strings.Contains(linkURL, "aliyundrive.com") || strings.Contains(linkURL, "alipan.com"):
		return "aliyun"
	case strings.Contains(linkURL, "pan.xunlei.com"):
		return "xunlei"
	case strings.Contains(linkURL, "cloud.189.cn"):
		return "tianyi"
	case strings.Contains(linkURL, "115.com") || strings.Contains(linkURL, "115cdn.com") || strings.Contains(linkURL, "anxia.com"):
		return "115"
	case strings.Contains(linkURL, "123684.com") || strings.Contains(linkURL, "123685.com") || 
		strings.Contains(linkURL, "123912.com") || strings.Contains(linkURL, "123pan.com") || 
		strings.Contains(linkURL, "123pan.cn") || strings.Contains(linkURL, "123592.com"): 
		return "123"
	default:
		return "others"
	}
}

// extractPasswordFromURL 从URL提取提取码
func (p *GyingPlugin) extractPasswordFromURL(linkURL string) string {
	// 百度网盘: ?pwd=xxxx
	if strings.Contains(linkURL, "?pwd=") {
		re := regexp.MustCompile(`\?pwd=([a-zA-Z0-9]+)`)
		if matches := re.FindStringSubmatch(linkURL); len(matches) > 1 {
			return matches[1]
		}
	}
	
	// 115网盘: ?password=xxxx
	if strings.Contains(linkURL, "?password=") {
		re := regexp.MustCompile(`\?password=([a-zA-Z0-9]+)`)
		if matches := re.FindStringSubmatch(linkURL); len(matches) > 1 {
			return matches[1]
		}
	}

	return ""
}

// deduplicateResults 去重
func (p *GyingPlugin) deduplicateResults(results []model.SearchResult) []model.SearchResult {
	seen := make(map[string]bool)
	var deduplicated []model.SearchResult

	for _, result := range results {
		if !seen[result.UniqueID] {
			seen[result.UniqueID] = true
			deduplicated = append(deduplicated, result)
		}
	}

	return deduplicated
}

// ============ 工具函数 ============

// generateHash 生成hash
func (p *GyingPlugin) generateHash(username string) string {
	salt := os.Getenv("GYING_HASH_SALT")
	if salt == "" {
		salt = "pansou_gying_secret_2025"
	}
	data := username + salt
	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:])
}

// maskUsername 脱敏用户名
func (p *GyingPlugin) maskUsername(username string) string {
	if len(username) <= 2 {
		return username
	}
	if len(username) <= 4 {
		return username[:1] + "**" + username[len(username)-1:]
	}
	return username[:2] + "****" + username[len(username)-2:]
}

// isHexString 判断是否为十六进制
func (p *GyingPlugin) isHexString(s string) bool {
	for _, c := range s {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return false
		}
	}
	return true
}

// respondSuccess 成功响应
func respondSuccess(c *gin.Context, message string, data interface{}) {
	c.JSON(200, gin.H{
		"success": true,
		"message": message,
		"data":    data,
	})
}

// respondError 错误响应
func respondError(c *gin.Context, message string) {
	c.JSON(200, gin.H{
		"success": false,
		"message": message,
		"data":    nil,
	})
}

// ============ Cookie加密（可选） ============

func getEncryptionKey() []byte {
	key := os.Getenv("GYING_ENCRYPTION_KEY")
	if key == "" {
		key = "default-32-byte-key-change-me!"
	}
	return []byte(key)[:32]
}

func encryptCookie(plaintext string) (string, error) {
	key := getEncryptionKey()
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}

	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

func decryptCookie(encrypted string) (string, error) {
	key := getEncryptionKey()
	ciphertext, err := base64.StdEncoding.DecodeString(encrypted)
	if err != nil {
		return "", err
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return "", fmt.Errorf("ciphertext too short")
	}

	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", err
	}

	return string(plaintext), nil
}

// ============ Session保活 ============

// startSessionKeepAlive 启动session保活任务
func (p *GyingPlugin) startSessionKeepAlive() {
	// 首次启动后延迟3分钟再开始（避免启动时过多请求）
	time.Sleep(3 * time.Minute)
	
	// 立即执行一次保活
	p.keepAllSessionsAlive()
	
	// 每3分钟执行一次保活
	ticker := time.NewTicker(3 * time.Minute)
	for range ticker.C {
		p.keepAllSessionsAlive()
	}
}

// keepAllSessionsAlive 保持所有用户的session活跃
func (p *GyingPlugin) keepAllSessionsAlive() {
	count := 0
	
	p.users.Range(func(key, value interface{}) bool {
		user := value.(*User)
		
		// 只为active状态的用户保活
		if user.Status != "active" {
			return true
		}
		
		// 获取scraper实例
		scraperVal, exists := p.scrapers.Load(user.Hash)
		if !exists {
			return true
		}
		
		scraper, ok := scraperVal.(*cloudscraper.Scraper)
		if !ok || scraper == nil {
			return true
		}
		
		// 访问首页保持session活跃
		go func(s *cloudscraper.Scraper, username string) {
			resp, err := s.Get("https://www.gying.net/")
			if err == nil && resp != nil {
				resp.Body.Close()
				if DebugLog {
					fmt.Printf("[Gying] 💓 Session保活成功: %s (状态码: %d)\n", username, resp.StatusCode)
				}
			}
		}(scraper, user.UsernameMasked)
		
		count++
		return true
	})
	
	if DebugLog && count > 0 {
		fmt.Printf("[Gying] 💓 已为 %d 个用户执行session保活\n", count)
	}
}

// ============ 定期清理 ============

func (p *GyingPlugin) startCleanupTask() {
	ticker := time.NewTicker(24 * time.Hour)
	for range ticker.C {
		deleted := p.cleanupExpiredUsers()
		marked := p.markInactiveUsers()

		if deleted > 0 || marked > 0 {
			fmt.Printf("[Gying] 清理任务完成: 删除 %d 个过期用户, 标记 %d 个不活跃用户\n", deleted, marked)
		}
	}
}

func (p *GyingPlugin) cleanupExpiredUsers() int {
	deletedCount := 0
	now := time.Now()
	expireThreshold := now.AddDate(0, 0, -30)

	p.users.Range(func(key, value interface{}) bool {
		user := value.(*User)
		if user.Status == "expired" && user.LastAccessAt.Before(expireThreshold) {
			if err := p.deleteUser(user.Hash); err == nil {
				deletedCount++
			}
		}
		return true
	})

	return deletedCount
}

func (p *GyingPlugin) markInactiveUsers() int {
	markedCount := 0
	now := time.Now()
	inactiveThreshold := now.AddDate(0, 0, -90)

	p.users.Range(func(key, value interface{}) bool {
		user := value.(*User)
		if user.LastAccessAt.Before(inactiveThreshold) && user.Status != "expired" {
			user.Status = "expired"
			user.Cookie = ""

			if err := p.saveUser(user); err == nil {
				markedCount++
			}
		}
		return true
	})

	return markedCount
}


