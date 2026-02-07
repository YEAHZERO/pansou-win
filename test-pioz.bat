@echo off
chcp 65001 >nul
title Pioz Plugin Test

echo ================================
echo    Pioz Plugin Test
echo ================================
echo.

REM Configuration
set API_URL=http://localhost:8889
set USERNAME=admin
set PASSWORD=123456

echo [1] Testing Pioz website accessibility...
echo.

REM Test Pioz website
curl -I -s https://www.pioz.cn/ | findstr "200"
if %errorlevel% equ 0 (
    echo [OK] Pioz website is accessible
) else (
    echo [!] Pioz website may not be accessible
    echo Trying to access anyway...
)
echo.

echo [2] Logging in to PanSou...
echo.

REM Login and get token
curl -s -X POST "%API_URL%/api/auth/login" ^
  -H "Content-Type: application/json" ^
  -d "{\"username\":\"%USERNAME%\",\"password\":\"%PASSWORD%\"}" ^
  -o token.json 2>nul

if %errorlevel% neq 0 (
    echo [X] Login failed
    echo Please check:
    echo 1. Service is running (start.bat)
    echo 2. Port 8889 is correct
    echo 3. Username and password are correct
    pause
    exit /b 1
)

echo [OK] Login successful
echo.

REM Extract token
for /f "tokens=2 delims=:," %%a in ('type token.json ^| findstr "token"') do (
    set TOKEN=%%a
)
set TOKEN=%TOKEN:"=%
set TOKEN=%TOKEN: =%

echo [3] Testing search with different keywords...
echo.

REM Test 1: Popular movie
echo Test 1: Searching for "电影"...
curl -s -X POST "%API_URL%/api/search" ^
  -H "Authorization: Bearer %TOKEN%" ^
  -H "Content-Type: application/json" ^
  -d "{\"keyword\":\"电影\",\"channels\":[],\"concurrency\":5}" ^
  -o result1.json

echo [OK] Search completed
type result1.json | findstr "total"
echo.

REM Test 2: Specific movie
echo Test 2: Searching for "流浪地球"...
curl -s -X POST "%API_URL%/api/search" ^
  -H "Authorization: Bearer %TOKEN%" ^
  -H "Content-Type: application/json" ^
  -d "{\"keyword\":\"流浪地球\",\"channels\":[],\"concurrency\":5}" ^
  -o result2.json

echo [OK] Search completed
type result2.json | findstr "total"
echo.

REM Test 3: TV series
echo Test 3: Searching for "美剧"...
curl -s -X POST "%API_URL%/api/search" ^
  -H "Authorization: Bearer %TOKEN%" ^
  -H "Content-Type: application/json" ^
  -d "{\"keyword\":\"美剧\",\"channels\":[],\"concurrency\":5}" ^
  -o result3.json

echo [OK] Search completed
type result3.json | findstr "total"
echo.

echo [4] Analyzing results...
echo.

REM Count results from each test
for %%f in (result1.json result2.json result3.json) do (
    echo File: %%f
    type %%f | findstr "pioz"
    echo.
)

echo ================================
echo Test Summary
echo ================================
echo.
echo Test 1 (电影): See result1.json
echo Test 2 (流浪地球): See result2.json
echo Test 3 (美剧): See result3.json
echo.
echo All results saved to result*.json files
echo.

REM Cleanup
del token.json 2>nul

echo ================================
echo Test completed!
echo ================================
echo.

echo Next steps:
echo 1. Check result*.json files for detailed results
echo 2. If results are 0, check service logs
echo 3. Try different keywords
echo 4. Check network connectivity to pioz.cn
echo.

pause
