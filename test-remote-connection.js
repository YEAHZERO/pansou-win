#!/usr/bin/env node

/**
 * PanSou 官方服务连接测试脚本
 */

const https = require('https');
const http = require('http');

function makeRequest(url, options = {}) {
  return new Promise((resolve, reject) => {
    const client = url.startsWith('https:') ? https : http;
    
    const req = client.request(url, {
      method: options.method || 'GET',
      headers: {
        'Content-Type': 'application/json',
        'User-Agent': 'PanSou-Test-Client/1.0.0',
        ...options.headers
      },
      timeout: 10000
    }, (res) => {
      let data = '';
      res.on('data', chunk => data += chunk);
      res.on('end', () => {
        try {
          const parsed = JSON.parse(data);
          resolve({ status: res.statusCode, data: parsed });
        } catch (e) {
          resolve({ status: res.statusCode, data: data });
        }
      });
    });

    req.on('error', reject);
    req.on('timeout', () => reject(new Error('Request timeout')));

    if (options.body) {
      req.write(JSON.stringify(options.body));
    }
    
    req.end();
  });
}

async function testConnection() {
  console.log('🔍 PanSou 官方服务连接测试');
  console.log('=' .repeat(50));
  console.log('');

  const baseUrl = 'https://so.252035.xyz';
  
  try {
    // 1. 测试健康检查接口
    console.log('1️⃣  测试健康检查接口...');
    const healthResponse = await makeRequest(`${baseUrl}/api/health`);
    
    if (healthResponse.status === 200) {
      console.log('✅ 健康检查成功');
      console.log(`   状态: ${healthResponse.data.status}`);
      console.log(`   插件启用: ${healthResponse.data.plugins_enabled}`);
      console.log(`   插件数量: ${healthResponse.data.plugin_count || 0}`);
      console.log(`   频道数量: ${healthResponse.data.channels_count || 0}`);
    } else {
      console.log(`❌ 健康检查失败 (状态码: ${healthResponse.status})`);
      return;
    }

    console.log('');

    // 2. 测试认证接口
    console.log('2️⃣  测试认证接口...');
    
    // 尝试不带认证的搜索请求
    try {
      const searchResponse = await makeRequest(`${baseUrl}/api/search`, {
        method: 'POST',
        body: { kw: 'test' }
      });
      
      if (searchResponse.status === 401) {
        console.log('✅ 认证检查正常 - 需要登录');
        console.log('   服务器正确要求认证');
      } else if (searchResponse.status === 200) {
        console.log('⚠️  认证未启用 - 可以直接搜索');
        console.log('   这可能意味着服务器未启用认证');
      } else {
        console.log(`❓ 未知响应 (状态码: ${searchResponse.status})`);
      }
    } catch (error) {
      console.log(`❌ 搜索测试失败: ${error.message}`);
    }

    console.log('');

    // 3. 网络连接总结
    console.log('3️⃣  连接总结:');
    console.log('✅ 网络连接正常');
    console.log('✅ 服务器响应正常');
    console.log('✅ API 接口可访问');
    
    console.log('');
    console.log('🎯 下一步:');
    console.log('1. 运行配置脚本: node setup-remote.js');
    console.log('2. 或手动配置 MCP 服务');
    console.log('3. 在 Cherry Studio 中导入配置');
    console.log('4. 使用你的账号密码进行认证');

  } catch (error) {
    console.log('❌ 连接测试失败');
    console.log(`   错误: ${error.message}`);
    console.log('');
    console.log('🔧 可能的解决方案:');
    console.log('1. 检查网络连接');
    console.log('2. 确认服务器地址正确');
    console.log('3. 检查防火墙设置');
    console.log('4. 稍后重试');
  }
}

// 运行测试
testConnection().catch(console.error);