package main

import (
	"fmt"
	"math"
	"time"

	"gorm.io/gorm"
)

func round2(f float64) float64 { return math.Round(f*100) / 100 }

// 取业务发生时间生效的费率版本:最后一个 effective_from <= at 的版本
func resolveRateVersion(db *gorm.DB, at time.Time) (*RateVersion, error) {
	var rv RateVersion
	err := db.Where("effective_from <= ?", at).Order("effective_from DESC").First(&rv).Error
	if err != nil {
		return nil, fmt.Errorf("找不到 %s 生效的费率版本", at.Format("2006-01-02"))
	}
	return &rv, nil
}

func writeAudit(db *gorm.DB, waybillID *uint, actor *User, action, detail string) {
	db.Create(&AuditLog{
		WaybillID: waybillID, ActorID: actor.ID, ActorName: actor.Name,
		Action: action, Detail: detail, CreatedAt: time.Now(),
	})
}

// 刷新运单状态(根据回单情况)
func refreshWaybillStatus(db *gorm.DB, waybillID uint) {
	var wb Waybill
	if err := db.First(&wb, waybillID).Error; err != nil {
		return
	}
	var approved Receipt
	if err := db.Where("waybill_id = ? AND status = ?", waybillID, ReceiptApproved).First(&approved).Error; err == nil {
		if approved.ReceivedPieces < wb.PlannedPieces {
			wb.Status = WaybillPartialSigned
		} else {
			wb.Status = WaybillSigned
		}
	} else {
		var cnt int64
		db.Model(&Receipt{}).Where("waybill_id = ? AND status = ?", waybillID, ReceiptPending).Count(&cnt)
		if cnt > 0 {
			wb.Status = WaybillReceiptPending
		} else {
			wb.Status = WaybillPendingReceipt
		}
	}
	db.Model(&Waybill{}).Where("id = ?", wb.ID).Update("status", wb.Status)
}

func waybillStatusName(s string) string {
	switch s {
	case WaybillPendingReceipt:
		return "待回单"
	case WaybillReceiptPending:
		return "回单待审核"
	case WaybillSigned:
		return "已签收"
	case WaybillPartialSigned:
		return "部分签收"
	}
	return s
}
