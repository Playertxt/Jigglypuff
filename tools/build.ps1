# Builds Jigglypuff.exe on Windows. Needs Go 1.24+; TypeScript (npm i -g typescript) only to rebuild the interface.
$ErrorActionPreference = 'Stop'
Set-Location (Join-Path $PSScriptRoot '..')

if (Get-Command tsc -ErrorAction SilentlyContinue) {
  Write-Host '> interface'
  Remove-Item -Recurse -Force internal\web\dist -ErrorAction SilentlyContinue
  New-Item -ItemType Directory internal\web\dist | Out-Null
  tsc -p web\tsconfig.json
  Copy-Item web\static\* internal\web\dist\
} else {
  Write-Host '> tsc not found: using the interface that is already built'
}

python toolsmksyso.py cmdjigglypuff
Write-Host '> Jigglypuff.exe'
New-Item -ItemType Directory -Force dist | Out-Null
$env:CGO_ENABLED = '0'
go build -trimpath -ldflags '-s -w -H windowsgui' -o dist\Jigglypuff.exe .\cmd\jigglypuff
Write-Host 'Built dist\Jigglypuff.exe'
