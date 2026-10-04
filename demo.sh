#!/usr/bin/env bash
# 演示三大场景: 重复回单拦截 / 部分签收件数核对 / 司机越权审核拦截
# 用法: ./demo.sh  (会重置数据并重启服务)
set -e
cd "$(dirname "$0")"
BASE=http://localhost:8080

echo ">>> 重置数据并重启服务..."
pkill -x server 2>/dev/null || true
sleep 1
rm -rf ./data ./uploads && mkdir -p ./uploads
(cd server && go build -o ../bin/server . 2>/dev/null) || PATH="$HOME/sdk/go/bin:$PATH" sh -c 'cd server && go build -o ../bin/server .'
nohup env ADDR=:8080 DB_PATH=./data/logistics.db UPLOAD_DIR=./uploads WEB_DIST=./web/dist ./bin/server > /tmp/demo-server.log 2>&1 &
sleep 2

login() { curl -s -X POST $BASE/api/login -H 'Content-Type: application/json' -d "{\"username\":\"$1\"}" | python3 -c 'import sys,json;print(json.load(sys.stdin)["token"])'; }
DRIVER=$(login driver1); OPS=$(login ops1); FIN=$(login finance1)
echo "test image" > /tmp/demo-pod.jpg

echo
echo "════════════════ 场景 1: 重复回单不能生成第二次签收 ════════════════"
echo "运单 WB2026-0002 已签收(50件)且结算已支付,司机再次上传回单:"
curl -s -X POST $BASE/api/waybills/2/receipts -H "Authorization: Bearer $DRIVER" \
  -F "signed_pieces=50" -F "damaged_pieces=0" -F "claimed_sign_time=2026-09-12T14:30" \
  -F "stamp_status=stamped" -F "files=@/tmp/demo-pod.jpg" | python3 -m json.tool

echo
echo "════════════════ 场景 2: 部分签收件数核对(计划100 → 实签80,破损3) ════════════════"
echo "--- ① 司机补录回单(注意: 声称签收时间 2026-01-05,实际补录时间由服务器盖章) ---"
curl -s -X POST $BASE/api/waybills/1/receipts -H "Authorization: Bearer $DRIVER" \
  -F "signed_pieces=80" -F "damaged_pieces=3" -F "claimed_sign_time=2026-01-05T10:00" \
  -F "stamp_status=stamped" -F "note=客户仓库盘点短少20件,3件外箱破损" -F "files=@/tmp/demo-pod.jpg" \
  | python3 -c 'import sys,json; r=json.load(sys.stdin)["receipt"]; rid=r["id"]; c=r["claimed_sign_time"][:16]; e=r["entry_time"][:19]; print("回单RC-%s: 声称签收=%s  实际补录=%s(服务器时间)" % (rid, c, e))'
echo "--- ② 运营审核通过 ---"
curl -s -X POST $BASE/api/receipts/4/review -H "Authorization: Bearer $OPS" -H 'Content-Type: application/json' \
  -d '{"action":"approve","note":"照片与件数核对无误"}' | python3 -m json.tool
echo "--- ③ 财务生成调整单(业务发生日 2025-12-30,应匹配旧费率 V2025: 12元/件) ---"
curl -s -X POST $BASE/api/waybills/1/adjustments -H "Authorization: Bearer $FIN" \
  | python3 -c 'import sys,json; a=json.load(sys.stdin)["adjustment"]; print("调整单 %s: 80件×12元 − 3件×5元 = %s元 | 原结算 %s元 | 差额 %s元 | 费率版本 %s" % (a["adjust_no"], a["adjusted_amount"], a["original_amount"], a["difference"], a["rate_version"]["version"]))'
echo "--- ④ 核对: 原结算保持不变,运单实际签收=80,自动登记争议 ---"
curl -s $BASE/api/waybills/1 -H "Authorization: Bearer $FIN" \
  | python3 -c 'import sys,json; d=json.load(sys.stdin); w=d["waybill"]; s=w["settlement"]; print("运单状态=%s 实际签收=%s件 原结算=%s元(%s,未被覆盖)" % (w["status"], w["actual_pieces"], s["amount"], s["status"])); print("自动登记争议:", [x["title"] for x in d["disputes"]])'

echo
echo "════════════════ 场景 3: 司机越权审核自己的补录被拦截 ════════════════"
echo "运单 WB2026-0003 有司机张师傅自己上传的待审回单(RC-2),他用司机账号尝试审核:"
curl -s -w "\nHTTP状态码: %{http_code}\n" -X POST $BASE/api/receipts/2/review \
  -H "Authorization: Bearer $DRIVER" -H 'Content-Type: application/json' -d '{"action":"approve"}'
echo "--- 运营账号可以正常审核 ---"
curl -s -X POST $BASE/api/receipts/2/review -H "Authorization: Bearer $OPS" -H 'Content-Type: application/json' \
  -d '{"action":"approve","note":"整单签收,回单清晰"}' | python3 -m json.tool

echo
echo ">>> 演示完成。浏览器打开 http://localhost:8080 查看界面(账号 driver1/ops1/finance1)"
