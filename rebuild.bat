@echo off
echo ========================================
echo 重新编译 PanSou
echo ========================================
echo.

echo 正在编译...
go build -o pansou.exe main.go

if %ERRORLEVEL% EQU 0 (
    echo.
    echo ========================================
    echo 编译成功！
    echo ========================================
    echo.
    echo 可执行文件: pansou.exe
    echo.
    echo 运行命令: pansou.exe
    echo 或使用: start-pansou.bat
    echo.
) else (
    echo.
    echo ========================================
    echo 编译失败！请检查错误信息
    echo ========================================
    echo.
)

pause
