@echo off
setlocal
if not exist "%~dp0Launch.ps1" (
    echo The launcher is missing. Extract the entire ZIP, then run START.cmd from the extracted folder.
    pause
    exit /b 1
)
powershell.exe -NoLogo -NoProfile -STA -ExecutionPolicy Bypass -File "%~dp0Launch.ps1" %*
if errorlevel 1 (
    pause
    exit /b 1
)
