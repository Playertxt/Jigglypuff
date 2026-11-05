#!/usr/bin/env bash
# Builds Jigglypuff: the web interface, Windows resources, the Windows exe and a demo build for this OS.
set -euo pipefail
cd "$(dirname "$0")/.."
VERSION="${VERSION:-1.0.0}"

echo "› interface"
rm -rf internal/web/dist && mkdir -p internal/web/dist
tsc -p web/tsconfig.json
cp web/static/* internal/web/dist/

echo "› resources"
python3 tools/mkicon.py web/static/icon.svg cmd/jigglypuff/jigglypuff.ico
(cd cmd/jigglypuff && llvm-windres-18 --target=pe-x86-64 -i jigglypuff.rc -o rsrc_windows_amd64.syso)

echo "› windows exe"
mkdir -p dist
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -H windowsgui -X main.version=${VERSION}" -o dist/Jigglypuff.exe ./cmd/jigglypuff

echo "› demo build"
CGO_ENABLED=0 go build -trimpath -ldflags "-X main.version=${VERSION}" -o dist/jigglypuff-demo ./cmd/jigglypuff
ls -la dist
