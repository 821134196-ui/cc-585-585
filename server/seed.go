package main

import (
	"log"
	"time"

	"gorm.io/gorm"
)

func seed(db *gorm.DB) {
	var cnt int64
	db.Model(&User{}).Count(&cnt)
	if cnt > 0 {
		return
	}
	log.Println("初始化种子数据...")

	// 用户: 司机/运营/财务
	driver1 := User{Username: "driver1", Name: "张师傅", Role: "driver"}
	driver2 := User{Username: "driver2", Name: "李师傅", Role: "driver"}
	ops := User{Username: "ops1", Name: "王运营", Role: "ops"}
	finance := User{Username: "finance1", Name: "赵财务", Role: "finance"}
	db.Create(&driver1)
	db.Create(&driver2)
	db.Create(&ops)
	db.Create(&finance)

	// 费率版本: 2025 旧版 / 2026 现行版
	v2025 := RateVersion{
		Version: "V2025", EffectiveFrom: time.Date(2025, 1, 1, 0, 0, 0, 0, time.Local),
		EffectiveTo: ptrTime(time.Date(2025, 12, 31, 23, 59, 59, 0, time.Local)),
		UnitPrice: 12.0, DamageDeduction: 5.0, Remark: "2025 年度合同费率",
	}
	v2026 := RateVersion{
		Version: "V2026", EffectiveFrom: time.Date(2026, 1, 1, 0, 0, 0, 0, time.Local),
		UnitPrice: 15.0, DamageDeduction: 8.0, Remark: "2026 年度合同费率(单价上调)",
	}
	db.Create(&v2025)
	db.Create(&v2026)

	mkSettlement := func(wid uint, rv RateVersion, pieces int, paid bool) Settlement {
		s := Settlement{
			WaybillID: wid, RateVersionID: rv.ID, Pieces: pieces,
			UnitPrice: rv.UnitPrice, Amount: round2(float64(pieces) * rv.UnitPrice),
			Status: "unpaid",
		}
		if paid {
			now := time.Now().Add(-48 * time.Hour)
			s.Status = "paid"
			s.PaidAt = &now
			s.PayTradeNo = "PAY202609120930000001"
		}
		return s
	}

	// 场景1: 部分签收核对 —— 业务发生在 2025-12-30(旧费率 V2025),计划 100 件,待司机补录
	w1 := Waybill{
		WaybillNo: "WB2026-0001", DriverID: driver1.ID, CustomerName: "杭州华美超市",
		RouteFrom: "上海", RouteTo: "杭州", PlannedPieces: 100,
		BusinessDate: time.Date(2025, 12, 30, 10, 0, 0, 0, time.Local),
		Status:       "in_transit",
	}
	db.Create(&w1)
	s := mkSettlement(w1.ID, v2025, 100, false)
	db.Create(&s)

	// 场景2: 重复回单拦截 —— 已签收且结算已付款,司机再传回单应被拒
	w2 := Waybill{
		WaybillNo: "WB2026-0002", DriverID: driver1.ID, CustomerName: "宁波优品仓",
		RouteFrom: "上海", RouteTo: "宁波", PlannedPieces: 50,
		BusinessDate: time.Date(2026, 9, 10, 9, 0, 0, 0, time.Local),
		Status:       "signed",
	}
	db.Create(&w2)
	actual50 := 50
	db.Model(&w2).Updates(map[string]interface{}{"actual_pieces": actual50})
	s = mkSettlement(w2.ID, v2026, 50, true)
	db.Create(&s) // 已付款
	reviewedAt := time.Now().Add(-72 * time.Hour)
	db.Create(&Receipt{
		WaybillID: w2.ID, UploaderID: driver1.ID,
		ClaimedSignTime: time.Date(2026, 9, 12, 14, 30, 0, 0, time.Local),
		EntryTime:       time.Now().Add(-96 * time.Hour),
		SignedPieces:    50, DamagedPieces: 0, StampStatus: "stamped",
		Note: "客户正常签收", Status: "approved",
		ReviewerID: &ops.ID, ReviewedAt: &reviewedAt, ReviewNote: "回单清晰,件数一致",
	})

	// 场景3: 司机越权审核拦截 —— driver1 已提交待审回单,尝试自己审核应被 403 拒绝
	w3 := Waybill{
		WaybillNo: "WB2026-0003", DriverID: driver1.ID, CustomerName: "苏州联合商城",
		RouteFrom: "上海", RouteTo: "苏州", PlannedPieces: 200,
		BusinessDate: time.Date(2026, 9, 20, 8, 0, 0, 0, time.Local),
		Status:       "receipt_pending",
	}
	db.Create(&w3)
	s = mkSettlement(w3.ID, v2026, 200, false)
	db.Create(&s)
	db.Create(&Receipt{
		WaybillID: w3.ID, UploaderID: driver1.ID,
		ClaimedSignTime: time.Date(2026, 9, 22, 16, 0, 0, 0, time.Local),
		EntryTime:       time.Now().Add(-2 * time.Hour),
		SignedPieces:    200, DamagedPieces: 0, StampStatus: "stamped",
		Note: "整单签收", Status: "pending",
	})

	// 场景4: 客户拒盖章 + 部分签收 —— 已审核通过,自动登记了争议
	w4 := Waybill{
		WaybillNo: "WB2026-0004", DriverID: driver2.ID, CustomerName: "南京鲜达冷链",
		RouteFrom: "上海", RouteTo: "南京", PlannedPieces: 60,
		BusinessDate: time.Date(2026, 9, 25, 11, 0, 0, 0, time.Local),
		Status:       "signed",
	}
	db.Create(&w4)
	actual58 := 58
	db.Model(&w4).Updates(map[string]interface{}{"actual_pieces": actual58, "damaged_pieces": 2})
	s = mkSettlement(w4.ID, v2026, 60, false)
	db.Create(&s)
	reviewedAt4 := time.Now().Add(-24 * time.Hour)
	rc4 := Receipt{
		WaybillID: w4.ID, UploaderID: driver2.ID,
		ClaimedSignTime: time.Date(2026, 9, 27, 10, 0, 0, 0, time.Local),
		EntryTime:       time.Now().Add(-30 * time.Hour),
		SignedPieces:    58, DamagedPieces: 2, StampStatus: "refused",
		Note: "客户称 2 件破损拒盖章,已拍照留证", Status: "approved",
		ReviewerID: &ops.ID, ReviewedAt: &reviewedAt4, ReviewNote: "照片与件数核对无误,按实际签收登记",
	}
	db.Create(&rc4)
	db.Create(&Dispute{
		DisputeNo: "DSP-SEED-001", WaybillID: w4.ID, Type: "pieces", Title: "部分签收",
		Detail: "计划 60 件,实际签收 58 件,短少 2 件,待与客户核实。", Status: "open", CreatedByID: ops.ID,
	})
	db.Create(&Dispute{
		DisputeNo: "DSP-SEED-002", WaybillID: w4.ID, Type: "damage", Title: "货物破损",
		Detail: "签收 58 件中破损 2 件,涉及运费扣减与理赔。", Status: "open", CreatedByID: ops.ID,
	})
	db.Create(&Dispute{
		DisputeNo: "DSP-SEED-003", WaybillID: w4.ID, Type: "pieces", Title: "客户拒绝盖章",
		Detail: "客户拒在回单上盖章,签收事实以照片与补录记录为准,待运营与客户确认。", Status: "open", CreatedByID: ops.ID,
	})

	log.Println("种子数据完成: 4 个运单 / 2 个费率版本 / 4 个用户")
}

func ptrTime(t time.Time) *time.Time { return &t }
