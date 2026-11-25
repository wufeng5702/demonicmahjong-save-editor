param(
    [ValidatePattern('^[0-9A-Za-z][0-9A-Za-z._+-]*$')][string]$Version = 'dev',
    [string]$OutputDir = 'bin',
    [switch]$Run
)

$ErrorActionPreference = 'Stop'
Set-Location $PSScriptRoot

go test ./... -count=1
if ($LASTEXITCODE -ne 0) { throw 'Go tests failed.' }

if (-not (Get-Command node -ErrorAction SilentlyContinue)) { throw 'Node.js 24+ is required to validate the frontend before building.' }
node --check web/app.js
if ($LASTEXITCODE -ne 0) { throw 'JavaScript syntax check failed.' }
node --test tests/model.test.mjs
if ($LASTEXITCODE -ne 0) { throw 'UI logic tests failed.' }
go vet ./...
if ($LASTEXITCODE -ne 0) { throw 'Go vet failed.' }

New-Item -ItemType Directory -Path $OutputDir -Force | Out-Null
$output = Join-Path $OutputDir 'DemonicMahjongSaveEditor.exe'
go build -trimpath -tags production -ldflags "-s -w -H windowsgui -X main.version=$Version" -o $output .
if ($LASTEXITCODE -ne 0) { throw 'EXE build failed.' }

# Include the licenses of modules actually linked into the executable, not SDK tooling.
$modules = go list -deps -tags production -f '{{if .Module}}{{.Module.Path}}|{{.Module.Version}}|{{.Module.Dir}}{{end}}' .
if ($LASTEXITCODE -ne 0) { throw 'Dependency license inventory failed.' }
$ownModule = go list -m
$goroot = go env GOROOT
$notices = [System.Collections.Generic.List[string]]::new()
$notices.Add('Go standard library')
$notices.Add((Get-Content -LiteralPath (Join-Path $goroot 'LICENSE') -Raw))
foreach ($module in ($modules | Where-Object { $_ } | Sort-Object -Unique)) {
    $parts = $module -split '\|', 3
    if ($parts[0] -eq $ownModule) { continue }
    $licenses = Get-ChildItem -LiteralPath $parts[2] -File | Where-Object { $_.Name -match '^(LICENSE|COPYING|NOTICE)([._-].*)?$' }
    if (-not $licenses) { throw "Dependency license missing: $($parts[0])" }
    $notices.Add("$($parts[0]) $($parts[1])")
    foreach ($license in $licenses) { $notices.Add((Get-Content -LiteralPath $license.FullName -Raw)) }
}
$noticesPath = Join-Path $OutputDir 'THIRD_PARTY_NOTICES.txt'
[System.IO.File]::WriteAllText([System.IO.Path]::GetFullPath($noticesPath), ($notices -join "`n`n"), [System.Text.UTF8Encoding]::new($false))
Write-Output "Built: $([System.IO.Path]::GetFullPath($output))"
if ($Run) { Start-Process -FilePath ([System.IO.Path]::GetFullPath($output)) }
