# 物流司机回单补录与运费争议系统

司机补交签收证据 → 运营核验 → 财务按合同费率版本调整运费,全程留痕。
任何争议都能查清来自**件数、破损还是费率**。

## 一条命令启动

```bash
./start.sh          # 首次自动装依赖、构建前端、编译并启动后端
./start.sh --rebuild  # 强制重新构建前端
```

访问 http://localhost:8080

- 后端:Go + Gin + GORM,SQLite 存于 `data/app.db`
- 前端:Vue 3 + Vite + Element Plus,构建产物 `web/dist`(由后端直接托管)
- 附件:本地 `uploads/` 目录;扫描(OCR)与支付均为本地模拟接口

## 演示账号(密码均 123456)

| 账号 | 角色 | 能做什么 |
|---|---|---|
| driver1 / driver2 | 司机 | 补录自己运单的回单、上传附件、发起争议 |
| ops1 | 运营 | 审核回单(不能审自己提交的)、处理争议 |
| finance1 | 财务 | 生成调整单、模拟支付、处理争议 |
| admin | 管理员 | 全部权限 |

## 三大演示场景(种子数据已备好)

### ① 部分签收件数核对 — 运单 WB20250310001(计划 100 件,合同运费 ¥1300)
1. 登录 `driver1` → 运单详情 → 「模拟扫描回单」自动识别出 **实收 95 / 破损 3** → 提交补录
2. 登录 `ops1` → 审核通过
3. 登录 `finance1` → 「按费率版本生成调整单」:系统按业务发生日(2025-03-10)命中 **V2025H1**(13 元/件、破损扣 10 元/件)
   - 应付 = 95×13 − 3×10 = **1205**;调整 = 1205 − 1300 = **−95 元**,计算依据随单保存
   - 若提交的预期金额与系统计算不一致 → 422 拒绝(前后端双重核对)

### ② 重复回单 + 司机越权审核 — 运单 WB20250805002(已有待审回单)
- `driver1` 再次提交该运单回单 → **409 不能生成第二次签收**;同一幂等键重复提交 → 返回原回单(防双击)
- `driver1` 点击「审核通过」自己提交的回单 → **403 拦截**(司机无审核权限;任何角色都不能审自己提交的回单)
- 申报签收时间填未来时刻 → **400 疑似伪造**;补录时间一律以服务器记录为准

### ③ 已付款后发现差额 — 运单 WB20240615003(已付 ¥960,V2024 费率)
- 客户对账回执仅 76 件 → 争议单 DP20240620001 留痕(处理记录可查)
- `finance1` 按争议核实件数(76 件 / 4 件破损)生成调整单:应付 = 76×12 − 4×8 = 880,调整 **−80 元**
- **原结算 960 元记录保持不变**(数据库触发器兜底,直接 UPDATE 金额会被拒绝);调整逐笔追加

## 关键设计

| 需求 | 实现 |
|---|---|
| 补录时间不可伪造 | `recorded_at` 由服务器写入;司机申报的 `declared_signed_at` 仅作参考,晚于当前时间拒绝,滞后超 24h 页面标记 |
| 重复回单不能二次签收 | 应用层 409 + 幂等键唯一索引 + 部分唯一索引(每运单最多一条已审回单)三重防护 |
| 司机不能审自己的补录 | 审核接口限 ops/admin 角色,且审核人 ≠ 提交人 |
| 费率版本可查 | `GET /api/rate-versions/resolve?at=日期`;调整单保存版本快照 + 计算依据 |
| 不覆盖原结算 | 结算金额无更新接口 + SQLite 触发器禁止改金额;差额走 Adjustment 追加 |
| 调整金额与件数对得上 | 服务端按费率版本重算,`expected_payable` 不一致返回 422 |
| 部分签收/拒盖章/付款后差额 | 自动生成争议线索,处理记录(DisputeHandling)全程留痕 |

## API 一览

```
POST /api/login                        登录
GET  /api/waybills                     运单列表(status/q/mine 过滤)
GET  /api/waybills/:id                 运单详情(回单/结算/调整/争议/日志)
POST /api/waybills/:id/receipts        司机补录回单(multipart,幂等键)
POST /api/receipts/:id/review          运营审核(approve/note)
GET  /api/rate-versions                费率版本列表
GET  /api/rate-versions/resolve?at=    查询某日适用费率版本
POST /api/waybills/:id/adjustments     财务生成调整单(按业务时费率)
POST /api/settlements/:id/pay          模拟支付
POST /api/scan/mock                    模拟扫描识别回单
POST /api/waybills/:id/disputes        发起争议
POST /api/disputes/:id/handlings       争议处理记录
GET  /api/audit-logs                   操作日志
```

## 目录结构

```
├── start.sh            # 一条命令启动
├── server/             # Go 后端(Gin + GORM + SQLite)
│   ├── main.go         # 路由、迁移、唯一索引与触发器
│   ├── models.go       # 运单/回单/结算/调整/争议/日志模型
│   ├── handlers.go     # 业务规则:防伪造、幂等、越权拦截、费率核对
│   └── seed.go         # 演示数据
├── web/                # Vue3 + Element Plus 前端
├── data/app.db         # SQLite(运行时生成)
└── uploads/            # 回单附件(运行时生成)
```
