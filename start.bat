@echo off
cd /d "%~dp0"
if not exist "bin\DemonicMahjongSaveEditor-v0.1.exe" (
  echo Please build first: powershell -File build.ps1
  pause
  exit /b 1
)
start "" "bin\DemonicMahjongSaveEditor-v0.1.exe" %*
