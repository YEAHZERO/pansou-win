@echo off
chcp 65001 >nul
title 最终重新编译和测试

echo ================================
echo   最终重新编译和测试
echo ================================
echo:

echo [1] 停止旧服务...
taskkill /F /IM pansou.exe >nul 2>&1
timeout /t 2 /nobreak >nul
echo [OK] 服务已停止
echo:

echo [2] 清除所有缓存...
rd /s /q cache >nul 2>&1
mkdir cache >nul 2>&1
echo [OK] 缓存已清除
echo:

echo [3] 重新编译...
go build -o pansou.exe
if %errorlevel% neq 0 (
    echo [X] 编译失败
    pause
    exit /b 1
)
echo [OK] 编译成功
echo:

echo [4] 启动服务...
start "PanSou Service" cmd /c start-simple.bat
echo [OK] 服务启动中...
echo:

echo [5] 等待服务就绪...
timeout /t 6 /nobreak >nul
echo:

echo [6] 测试搜索（关键词：太奶奶）...
echo:

REM Login
curl -s -X POST http://localhost:8889/api/auth/login -H "Content-Type: application/json" -d "{\"username\":\"admin\",\"password\":\"123456\"}" -o login.json

REM Extract token
for /f "tokens=2 delims=:," %%a in ('type login.json ^| findstr "token"') do set TOKEN=%%a
set TOKEN=%TOKEN:"=%
set TOKEN=%TOKEN: =%

REM Search
curl -X POST http://localhost:8889/api/search -H "Content-Type: application/json" -H "Authorization: Bearer %TOKEN%" -d "{\"keyword\":\"太奶奶\"}"

echo:
echo:
del login.json 2>nul

echo ================================
echo 测试完成！
echo ================================
echo:
echo 如果看到 total 大于 0，说明修复成功！
echo 如果仍然是 0，请检查服务日志。
echo:
pause
