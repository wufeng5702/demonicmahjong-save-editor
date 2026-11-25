$ErrorActionPreference = 'Stop'
Set-Location $PSScriptRoot

go test ./...
if ($LASTEXITCODE -ne 0) { throw 'Go tests failed.' }

if (Get-Command node -ErrorAction SilentlyContinue) {
    node --check web/app.js
    if ($LASTEXITCODE -ne 0) { throw 'JavaScript syntax check failed.' }
    node --test tests/model.test.mjs
    if ($LASTEXITCODE -ne 0) { throw 'UI logic tests failed.' }
}

New-Item -ItemType Directory -Path bin -Force | Out-Null
go build -tags production -ldflags '-s -w -H windowsgui' -o bin/DemonicMahjongSaveEditor-v0.1.exe .
if ($LASTEXITCODE -ne 0) { throw 'EXE build failed.' }
Write-Output "Built: $PSScriptRoot\bin\DemonicMahjongSaveEditor-v0.1.exe"
