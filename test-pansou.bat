@echo off
chcp 65001 >nul
title PanSou 服务测试

echo ================================
echo    PanSou 服务连接测试
echo ================================
echo.

set SERVER_URL=http://localhost:8888

echo ?? 测试服务器连接...
echo 服务地址: %SERVER_URL%
echo.

REM 测试健康检查接口
echo 1?? 健康检查测试:
curl -s -w "状态码: %%{http_code}\n" %SERVER_URL%/api/health

if %errorlevel% equ 0 (
    echo ? 健康检查成功
) else (
    echo ? 健康检查失败
    echo.
    echo 可能的原因:
    echo 1. 服务未启动 - 请运行 start-pansou.bat
    echo 2. 端口被占用 - 检查端口 8888
    echo 3. 防火墙阻止 - 检查防火墙设置
    goto :end
)

echo.

REM 测试搜索接口
echo 2?? 搜索接口测试:
echo 发送测试搜索请求...

curl -s -X POST ^
  -H "Content-Type: application/json" ^
  -d "{\"kw\":\"test\",\"res\":\"merge\"}" ^
  -w "状态码: %%{http_code}\n" ^
  %SERVER_URL%/api/search

if %errorlevel% equ 0 (
    echo ? 搜索接口响应正常
) else (
    echo ? 搜索接口测试失败
)

echo.

REM 显示服务信息
echo 3?? 服务信息:
echo 本地访问: %SERVER_URL%
echo API 文档: %SERVER_URL%/api/health
echo.

echo ?? 测试完成！
echo.
echo 如果测试成功，你可以:
echo 1. 在浏览器中访问 %SERVER_URL%/api/health
echo 2. 配置 MCP 服务连接到此地址
echo 3. 开始使用搜索功能

:end
echo.
pause