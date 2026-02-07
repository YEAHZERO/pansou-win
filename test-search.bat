@echo off
chcp 65001 >nul
title PanSou Search Test

echo ================================
echo    PanSou Search Test
echo ================================
echo.

REM Configuration
set API_URL=http://localhost:8889
set USERNAME=admin
set PASSWORD=123456

echo Step 1: Login to get token...
echo.

REM Login and get token
curl -X POST "%API_URL%/api/auth/login" ^
  -H "Content-Type: application/json" ^
  -d "{\"username\":\"%USERNAME%\",\"password\":\"%PASSWORD%\"}" ^
  -o token.json 2>nul

if %errorlevel% neq 0 (
    echo [X] Login failed
    echo Please check:
    echo 1. Service is running
    echo 2. Port 8889 is correct
    echo 3. Username and password are correct
    pause
    exit /b 1
)

echo [OK] Login successful
echo.

REM Extract token (simple method for Windows)
for /f "tokens=2 delims=:," %%a in ('type token.json ^| findstr "token"') do (
    set TOKEN=%%a
)
REM Remove quotes and spaces
set TOKEN=%TOKEN:"=%
set TOKEN=%TOKEN: =%

echo Token: %TOKEN%
echo.

echo Step 2: Test search with keyword...
echo.

REM Test search
set KEYWORD=电影

echo Searching for: %KEYWORD%
echo.

curl -X POST "%API_URL%/api/search" ^
  -H "Authorization: Bearer %TOKEN%" ^
  -H "Content-Type: application/json" ^
  -d "{\"keyword\":\"%KEYWORD%\",\"concurrency\":10}" ^
  -o search_result.json

echo.
echo.
echo [OK] Search completed
echo.
echo Results saved to: search_result.json
echo.

REM Display result summary
echo Result Summary:
echo ================================
type search_result.json | findstr "total"
type search_result.json | findstr "pioz"
echo ================================
echo.

echo Full results in: search_result.json
echo.

REM Cleanup
del token.json 2>nul

echo.
echo Test completed!
echo.
pause
