# 物流司机回单补录与运费争议系统

司机补交签收证据 → 运营核验 → 财务按合同费率版本生成调整单,件数/破损/费率争议全程可查。

## 一条命令启动

```bash
./start.sh
```

首次运行会自动构建前端(Vue + Vite)并编译后端(Go + Gin + GORM + SQLite),然后启动在 **http://localhost:8080**。
前端有改动时用 `./start.sh --build` 强制重新构建。

演示三大场景(重置数据并自动跑一遍):

```bash
./demo.sh
```

## 演示账号(登录页点击即可进入)

| 账号 | 姓名 | 角色 | 能做什么 |
|---|---|---|---|
| `driver1` | 张师傅 | 司机 | 补录回单、上传照片,**不能审核** |
| `driver2` | 李师傅 | 司机 | 同上 |
| `ops1` | 王运营 | 运营 | 审核回单、登记/办结争议 |
| `finance1` | 赵财务 | 财务 | 生成调整单、模拟支付、查看费率版本 |

## 核心流程

1. **司机补录**:在运单详情页上传回单照片,填写实际签收件数、破损件数、客户签收时间(仅作声明)。
   **实际补录时间由服务器盖章**,无法伪造为原签收时刻,两个时间在页面上并列展示。
2. **运营核验**:审核通过才把签收件数登记到运单(只写一次);部分签收、破损、客户拒盖章会**自动登记争议**。
3. **财务调整**:按**原业务发生日**匹配费率版本生成调整单,保留完整计算依据(JSON);**原结算永不被覆盖**,差额走调整单单独支付。
4. **争议追溯**:运单详情页并列显示计划件数/实际签收/破损/合同运费,争议按件数、破损、费率分类。

## 三大演示场景(种子数据已内置)

| 场景 | 运单 | 操作 | 结果 |
|---|---|---|---|
| 重复回单拦截 | WB2026-0002(已签收已付款) | 司机再次上传回单 | **409** 重复回单不能生成第二次签收 |
| 部分签收核对 | WB2026-0001(计划100件,业务日2025-12-30) | 司机补录80件破损3件 → 运营通过 → 财务生成调整单 | 按旧费率 V2025:80×12−3×5=**945元**,原结算1200元保留,差额−255元,自动登记2条争议 |
| 司机越权审核 | WB2026-0003(张师傅自己的待审回单) | 司机账号调用审核 | **403** 拦截;运营账号可正常审核 |

## 业务规则与实现位置

| 规则 | 实现 |
|---|---|
| 补录时间防伪造 | `server/handlers.go` `uploadReceipt`:`EntryTime = time.Now()` 服务器写入,客户端时间只存为 `ClaimedSignTime` 声明字段 |
| 重复回单拦截 | 上传时检查已有 approved 回单(409);审核通过时事务内二次检查 |
| 司机不能审核自己的补录 | 角色中间件 `requireRole("ops")` + `reviewReceipt` 中 `UploaderID == 当前用户` 双重拦截(403) |
| 费率版本按业务发生时 | `rateVersionAt(waybill.BusinessDate)` 匹配生效区间,版本快照存入调整单 `Basis` |
| 调整金额与件数对账 | 调整金额 = 审核回单签收件数×单价 − 破损×扣减,件数直接取自审核通过的回单 |
| 不覆盖原结算 | `Settlement` 只读,调整单独立成表关联原结算;已付款后仍可补差额调整 |
| 部分签收/拒盖章留痕 | 审核通过时自动登记争议;回单保留盖章状态与照片附件 |

## 技术栈与目录

- 后端:Go + Gin + GORM + SQLite(纯 Go 驱动,免 CGO)
- 前端:Vue 3 + Vite + vue-router
- 附件:本地 `uploads/` 目录;扫码与支付为本地模拟接口(`POST /api/mock/scan`、`POST /api/mock/pay`)

```
server/    Go 后端(models / handlers / seed)
web/       Vue 前端(运单列表/详情、调整单、争议、费率版本)
data/      SQLite 数据库(首次启动自动建表+种子数据)
uploads/   回单照片附件
start.sh   一条命令启动
demo.sh    一条命令演示三大场景
```

## 主要 API

```
POST /api/login                        登录
GET  /api/waybills                     运单列表(司机只看自己的)
GET  /api/waybills/:id                 运单详情(结算/回单/调整/争议)
POST /api/waybills/:id/receipts        司机补录回单(multipart,附件+件数+声明签收时间)
POST /api/receipts/:id/review          运营审核(approve/reject)
POST /api/waybills/:id/adjustments     财务生成调整单(自动匹配费率版本)
GET  /api/rate-versions                费率版本查询
GET  /api/adjustments                  调整单列表
GET  /api/disputes                     争议列表
POST /api/waybills/:id/disputes        登记争议
POST /api/disputes/:id/resolve         办结争议
POST /api/mock/scan                    模拟扫码(运单号或 RC-<回单id>)
POST /api/mock/pay                     模拟支付(settlement/adjustment)
```
