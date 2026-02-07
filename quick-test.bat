@echo off
chcp 65001 >nul
title Quick Test

echo ================================
echo   Quick Test
echo ================================
echo:

REM Wait for service to start
echo Waiting for service to start...
timeout /t 3 /nobreak >nul

echo:
echo [1] Health Check
curl -s http://localhost:8889/api/health
echo:
echo:

echo [2] Login
curl -s -X POST http://localhost:8889/api/auth/login -H "Content-Type: application/json" -d "{\"username\":\"admin\",\"password\":\"123456\"}" -o token.json
echo:

if not exist token.json (
    echo [X] Login failed
    pause
    exit /b 1
)

echo [OK] Login successful
echo:

REM Extract token
for /f "tokens=2 delims=:," %%a in ('type token.json ^| findstr "token"') do set TOKEN=%%a
set TOKEN=%TOKEN:"=%
set TOKEN=%TOKEN: =%

echo [3] Search Test
curl -s -X POST http://localhost:8889/api/search -H "Authorization: Bearer %TOKEN%" -H "Content-Type: application/json" -d "{\"keyword\":\"test\",\"channels\":[],\"concurrency\":5}" -o result.json
echo:

echo [4] Results
type result.json
echo:
echo:

del token.json 2>nul
del result.json 2>nul

echo ================================
echo Test completed!
echo ================================
pause
