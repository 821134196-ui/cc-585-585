#!/usr/bin/env bash
# 一条命令启动: ./start.sh
set -e
cd "$(dirname "$0")"

# Go 环境(优先系统 Go,其次 ~/sdk/go)
if ! command -v go >/dev/null 2>&1; then
  if [ -x "$HOME/sdk/go/bin/go" ]; then
    export PATH="$HOME/sdk/go/bin:$PATH"
  else
    echo "未找到 Go,请先安装 Go 1.22+: https://go.dev/dl/"
    exit 1
  fi
fi

# 前端构建(dist 不存在或传入 --build 时)
if [ ! -d web/dist ] || [ "$1" = "--build" ]; then
  echo ">>> 构建前端..."
  (cd web && npm install && npm run build)
fi

mkdir -p data uploads

echo ">>> 编译后端..."
(cd server && go build -o ../bin/server .)

echo ">>> 启动服务 http://localhost:8080 (账号: driver1/driver2/ops1/finance1)"
exec env ADDR=${ADDR:-:8080} \
  DB_PATH=./data/logistics.db \
  UPLOAD_DIR=./uploads \
  WEB_DIST=./web/dist \
  ./bin/server
