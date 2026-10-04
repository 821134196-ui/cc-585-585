package main

import (
	"fmt"
	"time"

	"gorm.io/gorm"
)

func mustDate(s string) time.Time {
	t, err := time.ParseInLocation("2006-01-02", s, time.Local)
	if err != nil {
		panic(err)
	}
	return t
}

// seed 在空库时写入演示数据
func seed(db *gorm.DB) error {
	var count int64
	db.Model(&User{}).Count(&count)
	if count > 0 {
		return nil
	}

	users := []User{
		{Username: "driver1", Password: "123456", Name: "张师傅(司机)", Role: "driver", Token: "tok-driver1"},
		{Username: "driver2", Password: "123456", Name: "赵师傅(司机)", Role: "driver", Token: "tok-driver2"},
		{Username: "ops1", Password: "123456", Name: "李运营(运营)", Role: "ops", Token: "tok-ops1"},
		{Username: "finance1", Password: "123456", Name: "王财务(财务)", Role: "finance", Token: "tok-finance1"},
		{Username: "admin", Password: "123456", Name: "系统管理员", Role: "admin", Token: "tok-admin"},
	}
	if err := db.Create(&users).Error; err != nil {
		return err
	}

	rates := []RateVersion{
		{Version: "V2024", EffectiveFrom: mustDate("2024-01-01"), UnitPrice: 12, DamageDeductionPerPiece: 8, Remark: "2024 年度合同费率"},
		{Version: "V2025H1", EffectiveFrom: mustDate("2025-01-01"), UnitPrice: 13, DamageDeductionPerPiece: 10, Remark: "2025 上半年费率,单价上调"},
		{Version: "V2025H2", EffectiveFrom: mustDate("2025-07-01"), UnitPrice: 14, DamageDeductionPerPiece: 10, Remark: "2025 下半年费率,旺季调价"},
	}
	if err := db.Create(&rates).Error; err != nil {
		return err
	}

	driver1 := users[0]
	driver2 := users[1]

	waybills := []Waybill{
		// 场景1:部分签收 + 破损,演示件数核对与调整计算(V2025H1 费率)
		{WaybillNo: "WB20250310001", DriverID: driver1.ID, Customer: "华美超市", Origin: "上海", Destination: "杭州",
			BusinessDate: mustDate("2025-03-10"), PlannedPieces: 100, ContractFreight: 1300, Status: WaybillPendingReceipt},
		// 场景2:已有待审回单,演示重复回单拦截 + 司机越权审核拦截
		{WaybillNo: "WB20250805002", DriverID: driver1.ID, Customer: "鑫源电器", Origin: "苏州", Destination: "南京",
			BusinessDate: mustDate("2025-08-05"), PlannedPieces: 50, ContractFreight: 700, Status: WaybillReceiptPending},
		// 场景3:老费率运单,已结算已付款,之后发现差额 -> 争议 + 调整(不覆盖原结算)
		{WaybillNo: "WB20240615003", DriverID: driver2.ID, Customer: "百汇药房", Origin: "上海", Destination: "合肥",
			BusinessDate: mustDate("2024-06-15"), PlannedPieces: 80, ContractFreight: 960, Status: WaybillSigned},
		// 场景4:全新运单,走完整流程
		{WaybillNo: "WB20250920004", DriverID: driver1.ID, Customer: "乐购便利店", Origin: "杭州", Destination: "宁波",
			BusinessDate: mustDate("2025-09-20"), PlannedPieces: 30, ContractFreight: 420, Status: WaybillPendingReceipt},
	}
	if err := db.Create(&waybills).Error; err != nil {
		return err
	}

	// 每张运单一张原结算单
	for _, wb := range waybills {
		st := Settlement{WaybillID: wb.ID, Amount: wb.ContractFreight, Status: "unpaid", CreatedAt: wb.BusinessDate.Add(24 * time.Hour)}
		if err := db.Create(&st).Error; err != nil {
			return err
		}
	}

	// 场景2:driver1 已提交待审回单(用于演示:重复提交被拦截 / 司机审核自己被拦截)
	pendingReceipt := Receipt{
		WaybillID: waybills[1].ID, UploaderID: driver1.ID,
		ReceivedPieces: 50, DamagedPieces: 0,
		DeclaredSignedAt: mustDate("2025-08-06").Add(10 * time.Hour),
		RecordedAt:       mustDate("2025-08-08").Add(9 * time.Hour), // 补录晚于签收,系统如实记录
		Status:           ReceiptPending,
		IdempotencyKey:   "seed-wb002-receipt-1",
		Attachments:      "[]",
	}
	if err := db.Create(&pendingReceipt).Error; err != nil {
		return err
	}

	// 场景3:已审核回单 + 已付款结算 + 付款后发现差额的争议
	var st3 Settlement
	if err := db.Where("waybill_id = ?", waybills[2].ID).First(&st3).Error; err != nil {
		return err
	}
	approvedReceipt := Receipt{
		WaybillID: waybills[2].ID, UploaderID: driver2.ID,
		ReceivedPieces: 80, DamagedPieces: 0,
		DeclaredSignedAt: mustDate("2024-06-16").Add(14 * time.Hour),
		RecordedAt:       mustDate("2024-06-16").Add(18 * time.Hour),
		Status:           ReceiptApproved,
		ReviewerID:       &users[2].ID,
		ReviewedAt:       ptrTime(mustDate("2024-06-17").Add(9 * time.Hour)),
		ReviewNote:       "回单齐全,审核通过",
		IdempotencyKey:   "seed-wb003-receipt-1",
		Attachments:      "[]",
	}
	if err := db.Create(&approvedReceipt).Error; err != nil {
		return err
	}
	if err := db.Model(&st3).Update("status", "paid").Error; err != nil {
		return err
	}
	pay := Payment{SettlementID: st3.ID, Amount: st3.Amount, TxnNo: "PAY20240618000001", OperatorID: users[3].ID, PaidAt: mustDate("2024-06-18").Add(10 * time.Hour)}
	if err := db.Create(&pay).Error; err != nil {
		return err
	}
	// 已付款后发现差额:客户回执只有 76 件,与已付 80 件不符 -> 争议留痕
	dispute := Dispute{
		DisputeNo: "DP20240620001", WaybillID: waybills[2].ID, ReceiptID: &approvedReceipt.ID,
		Type: DisputePieces, Title: "已付款后发现件数差额",
		Description: "客户月度对账回执确认仅收到 76 件,与已结算 80 件差 4 件,待财务按 V2024 费率补调整单。",
		Status: DisputeProcessing, CreatorID: users[2].ID, CreatedAt: mustDate("2024-06-20").Add(11 * time.Hour),
	}
	if err := db.Create(&dispute).Error; err != nil {
		return err
	}
	handling := DisputeHandling{
		DisputeID: dispute.ID, Action: "联系客户核实",
		Note:       "客户仓库签收底单显示 6 月 16 日实收 76 件,4 件破损拒收退回。",
		OperatorID: users[2].ID, CreatedAt: mustDate("2024-06-21").Add(15 * time.Hour),
	}
	if err := db.Create(&handling).Error; err != nil {
		return err
	}

	audit := AuditLog{
		WaybillID: &waybills[2].ID, ActorID: users[2].ID, ActorName: users[2].Name,
		Action: "seed", Detail: "系统初始化演示数据",
		CreatedAt: time.Now(),
	}
	if err := db.Create(&audit).Error; err != nil {
		return err
	}

	fmt.Println("[seed] 演示数据已写入:5 用户 / 3 费率版本 / 4 运单")
	return nil
}

func ptrTime(t time.Time) *time.Time { return &t }
