@echo off
chcp 65001 >nul

echo 最终测试 - 关键词: 太奶奶
echo:

curl -s -X POST http://localhost:8889/api/auth/login -H "Content-Type: application/json" -d "{\"username\":\"admin\",\"password\":\"123456\"}" -o login.json

for /f "tokens=2 delims=:," %%a in ('type login.json ^| findstr "token"') do set TOKEN=%%a
set TOKEN=%TOKEN:"=%
set TOKEN=%TOKEN: =%

echo 搜索中...
curl -X POST http://localhost:8889/api/search -H "Content-Type: application/json" -H "Authorization: Bearer %TOKEN%" -d "{\"keyword\":\"太奶奶\"}"

echo:
echo:
del login.json 2>nul
pause
