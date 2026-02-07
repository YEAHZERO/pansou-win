@echo off
chcp 65001 >nul
title PanSou Service Test

echo ================================
echo    PanSou Service Test
echo ================================
echo.

set SERVER_URL=http://localhost:8889

echo Testing service...
echo Server URL: %SERVER_URL%
echo.

REM Test health endpoint
echo 1. Testing health endpoint:
curl -s -w "Status: %%{http_code}\n" %SERVER_URL%/api/health

if %errorlevel% equ 0 (
    echo [OK] Service is running
) else (
    echo [X] Service connection failed
    echo.
    echo Possible reasons:
    echo 1. Service not started - run start-pansou.bat
    echo 2. Port occupied - check port 8889
    echo 3. Firewall blocking - check firewall settings
    goto :end
)

echo.

REM Test search endpoint
echo 2. Testing search endpoint:
echo Sending test search request...

curl -s -X POST ^
  -H "Content-Type: application/json" ^
  -d "{\"kw\":\"test\",\"res\":\"merge\"}" ^
  -w "Status: %%{http_code}\n" ^
  %SERVER_URL%/api/search

if %errorlevel% equ 0 (
    echo [OK] Search endpoint responding
) else (
    echo [X] Search endpoint test failed
)

echo.

REM Display service info
echo 3. Service Information:
echo Access URL: %SERVER_URL%
echo API Docs: %SERVER_URL%/api/health
echo.

echo Test completed!
echo.
echo If tests passed, you can:
echo 1. Access service at %SERVER_URL%/api/health
echo 2. Connect MCP client to this address
echo 3. Start using the search service

:end
echo.
pause
