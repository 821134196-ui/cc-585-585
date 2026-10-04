#!/usr/bin/env bash
# 物流回单补录与运费争议系统 —— 一条命令启动
# 用法: ./start.sh           首次自动装依赖、构建前端、启动服务
#       ./start.sh --rebuild 强制重新构建前端
set -e
cd "$(dirname "$0")"
ROOT="$PWD"

# 1. 找到 Go
if command -v go >/dev/null 2>&1; then
  GO=go
elif [ -x /home/node/go-sdk/go/bin/go ]; then
  GO=/home/node/go-sdk/go/bin/go
elif [ -x /usr/local/go/bin/go ]; then
  GO=/usr/local/go/bin/go
else
  echo "未找到 Go,请先安装 Go 1.22+"; exit 1
fi
export PATH="$(dirname "$GO"):$PATH"
export GOPROXY="${GOPROXY:-https://goproxy.cn,direct}"
export GOSUMDB=off

# 2. 前端依赖与构建
cd "$ROOT/web"
if [ ! -d node_modules ]; then
  echo "[start] 安装前端依赖..."
  npm install --registry=https://registry.npmmirror.com
fi
if [ ! -d dist ] || [ "$1" = "--rebuild" ]; then
  echo "[start] 构建前端..."
  npm run build
fi

# 3. 编译并启动后端(服务前端静态文件 + API + 本地附件)
cd "$ROOT"
mkdir -p data uploads bin
echo "[start] 编译后端..."
(cd "$ROOT/server" && "$GO" build -o "$ROOT/bin/server" .)
echo "[start] 启动服务..."
exec ./bin/server
