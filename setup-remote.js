#!/usr/bin/env node

/**
 * PanSou 官方服务配置脚本
 * 帮助用户快速配置连接到 https://so.252035.xyz
 */

const fs = require('fs');
const path = require('path');
const readline = require('readline');

const rl = readline.createInterface({
  input: process.stdin,
  output: process.stdout
});

function question(prompt) {
  return new Promise((resolve) => {
    rl.question(prompt, resolve);
  });
}

async function main() {
  console.log('🚀 PanSou 官方服务配置向导');
  console.log('=' .repeat(50));
  console.log('');

  // 获取项目路径
  const currentDir = process.cwd();
  const defaultPath = path.join(currentDir, 'typescript', 'dist', 'index.js');
  
  console.log(`当前项目路径: ${currentDir}`);
  console.log(`TypeScript 构建路径: ${defaultPath}`);
  console.log('');

  // 检查 TypeScript 构建文件是否存在
  if (!fs.existsSync(defaultPath)) {
    console.log('❌ 未找到 TypeScript 构建文件！');
    console.log('请先运行以下命令构建项目：');
    console.log('  cd typescript');
    console.log('  npm install');
    console.log('  npm run build');
    console.log('');
    console.log('或者运行 Windows 安装向导：');
    console.log('  install-windows.bat');
    console.log('');
    process.exit(1);
  }

  console.log('✅ TypeScript 构建文件已找到');
  console.log('');

  // 获取用户输入
  const username = await question('请输入官方服务用户名 (默认: admin): ') || 'admin';
  const password = await question('请输入官方服务密码: ');
  
  if (!password) {
    console.log('❌ 密码不能为空！');
    process.exit(1);
  }

  const timeout = await question('请输入请求超时时间（秒，默认: 60): ') || '60';
  const maxResults = await question('请输入最大搜索结果数（默认: 100): ') || '100';

  console.log('');
  console.log('📝 生成配置文件...');

  // 生成配置
  const config = {
    mcpServers: {
      "pansou-remote": {
        command: "node",
        args: [defaultPath], // 使用原始路径，Windows 会自动处理
        env: {
          PANSOU_SERVER_URL: "https://so.252035.xyz",
          REQUEST_TIMEOUT: timeout,
          MAX_RESULTS: maxResults,
          DEFAULT_CLOUD_TYPES: "baidu,aliyun,quark,tianyi,uc,mobile,115,pikpak,xunlei,123,magnet,ed2k,others",
          AUTO_START_BACKEND: "false",
          DOCKER_MODE: "false",
          BACKEND_SHUTDOWN_DELAY: "0",
          BACKEND_STARTUP_TIMEOUT: "5000",
          IDLE_TIMEOUT: "600000",
          ENABLE_IDLE_SHUTDOWN: "false",
          PROJECT_ROOT_PATH: "",
          ENABLED_PLUGINS: "",
          AUTH_ENABLED: "true",
          AUTH_USERNAME: username,
          AUTH_PASSWORD: password,
          LOG_LEVEL: "info"
        }
      }
    },
    _comments: {
      description: "PanSou MCP服务 - 官方远程服务配置",
      version: "2.0",
      generated_at: new Date().toISOString(),
      服务地址: "https://so.252035.xyz",
      使用说明: [
        "1. 在 Cherry Studio 中导入此配置文件",
        "2. 启用 pansou-remote 服务",
        "3. 开始使用搜索功能"
      ],
      安全提醒: [
        "此文件包含密码信息，请妥善保管",
        "不要将此文件提交到版本控制系统",
        "定期更换密码以确保安全"
      ]
    }
  };

  // 写入配置文件
  const configPath = path.join(currentDir, 'mcp-config-remote-generated.json');
  fs.writeFileSync(configPath, JSON.stringify(config, null, 2), 'utf8');

  console.log('✅ 配置文件已生成！');
  console.log('');
  console.log('📁 配置文件路径:');
  console.log(`   ${configPath}`);
  console.log('');
  console.log('🎯 下一步操作:');
  console.log('1. 打开 Cherry Studio');
  console.log('2. 进入设置 → MCP');
  console.log('3. 选择"添加服务器" → "从JSON导入"');
  console.log('4. 复制配置文件内容并导入');
  console.log('5. 启用 pansou-remote 服务');
  console.log('6. 开始使用搜索功能！');
  console.log('');
  console.log('🔍 测试搜索示例:');
  console.log('   "帮我搜索速度与激情"');
  console.log('   "搜索Python教程"');
  console.log('');
  console.log('⚠️  安全提醒:');
  console.log('   配置文件包含密码，请妥善保管！');
  console.log('');

  rl.close();
}

main().catch(console.error);