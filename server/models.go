package main

import (
	"time"
)

// 角色: driver 司机 / ops 运营 / finance 财务
type User struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Username  string    `gorm:"uniqueIndex" json:"username"`
	Name      string    `json:"name"`
	Role      string    `json:"role"`
	CreatedAt time.Time `json:"created_at"`
}

// 运单
type Waybill struct {
	ID            uint        `gorm:"primaryKey" json:"id"`
	WaybillNo     string      `gorm:"uniqueIndex" json:"waybill_no"`
	DriverID      uint        `json:"driver_id"`
	Driver        User        `json:"driver"`
	CustomerName  string      `json:"customer_name"`
	RouteFrom     string      `json:"route_from"`
	RouteTo       string      `json:"route_to"`
	PlannedPieces int         `json:"planned_pieces"`
	ActualPieces  *int        `json:"actual_pieces"` // 审核通过后写入,只写一次
	DamagedPieces int         `json:"damaged_pieces"`
	BusinessDate  time.Time   `json:"business_date"` // 原业务发生时间,决定费率版本
	Status        string      `json:"status"`        // in_transit / receipt_pending / signed / adjusted
	CreatedAt     time.Time   `json:"created_at"`
	Settlement    *Settlement `json:"settlement,omitempty"`
	Receipts      []Receipt   `json:"receipts,omitempty"`
}

// 原始结算: 生成后不可覆盖,后续差额只能走调整单
type Settlement struct {
	ID            uint        `gorm:"primaryKey" json:"id"`
	WaybillID     uint        `gorm:"uniqueIndex" json:"waybill_id"` // 一单一结算
	RateVersionID uint        `json:"rate_version_id"`
	RateVersion   RateVersion `json:"rate_version"`
	Pieces        int         `json:"pieces"`
	UnitPrice     float64     `json:"unit_price"`
	Amount        float64     `json:"amount"`
	Status        string      `json:"status"` // unpaid / paid
	PaidAt        *time.Time  `json:"paid_at"`
	PayTradeNo    string      `json:"pay_trade_no"`
	CreatedAt     time.Time   `json:"created_at"`
}

// 回单(补录)
type Receipt struct {
	ID              uint         `gorm:"primaryKey" json:"id"`
	WaybillID       uint         `json:"waybill_id"`
	UploaderID      uint         `json:"uploader_id"`
	Uploader        User         `json:"uploader"`
	ClaimedSignTime time.Time    `json:"claimed_sign_time"` // 司机声称的签收时间,仅作声明
	EntryTime       time.Time    `json:"entry_time"`        // 实际补录时间,服务器盖章,不可伪造
	SignedPieces    int          `json:"signed_pieces"`
	DamagedPieces   int          `json:"damaged_pieces"`
	StampStatus     string       `json:"stamp_status"` // stamped / refused(客户拒盖章)
	Note            string       `json:"note"`
	Status          string       `json:"status"` // pending / approved / rejected
	ReviewerID      *uint        `json:"reviewer_id"`
	Reviewer        *User        `json:"reviewer,omitempty"`
	ReviewedAt      *time.Time   `json:"reviewed_at"`
	ReviewNote      string       `json:"review_note"`
	Attachments     []Attachment `json:"attachments,omitempty"`
	CreatedAt       time.Time    `json:"created_at"`
}

type Attachment struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	ReceiptID   uint      `json:"receipt_id"`
	FileName    string    `json:"file_name"`
	StoredName  string    `json:"stored_name"`
	ContentType string    `json:"content_type"`
	Size        int64     `json:"size"`
	CreatedAt   time.Time `json:"created_at"`
}

// 费率版本: 按原业务发生时间匹配,财务可查
type RateVersion struct {
	ID              uint       `gorm:"primaryKey" json:"id"`
	Version         string     `gorm:"uniqueIndex" json:"version"`
	EffectiveFrom   time.Time  `json:"effective_from"`
	EffectiveTo     *time.Time `json:"effective_to"`
	UnitPrice       float64    `json:"unit_price"`       // 每件运费
	DamageDeduction float64    `json:"damage_deduction"` // 每件破损扣减
	Remark          string     `json:"remark"`
	CreatedAt       time.Time  `json:"created_at"`
}

// 调整单: 财务按业务发生时费率版本生成,保留计算依据
type Adjustment struct {
	ID              uint        `gorm:"primaryKey" json:"id"`
	AdjustNo        string      `gorm:"uniqueIndex" json:"adjust_no"`
	WaybillID       uint        `json:"waybill_id"`
	Waybill         Waybill     `json:"waybill"`
	ReceiptID       uint        `json:"receipt_id"`
	SettlementID    uint        `json:"settlement_id"`
	RateVersionID   uint        `json:"rate_version_id"`
	RateVersion     RateVersion `json:"rate_version"`
	SignedPieces    int         `json:"signed_pieces"`
	DamagedPieces   int         `json:"damaged_pieces"`
	UnitPrice       float64     `json:"unit_price"`
	DamageDeduction float64     `json:"damage_deduction"`
	AdjustedAmount  float64     `json:"adjusted_amount"`
	OriginalAmount  float64     `json:"original_amount"`
	Difference      float64     `json:"difference"`
	Basis           string      `json:"basis"` // JSON 计算依据
	Status          string      `json:"status"` // confirmed / paid
	PayTradeNo      string      `json:"pay_trade_no"`
	PaidAt          *time.Time  `json:"paid_at"`
	CreatedByID     uint        `json:"created_by_id"`
	CreatedBy       User        `json:"created_by"`
	CreatedAt       time.Time   `json:"created_at"`
}

// 争议: 件数 / 破损 / 费率
type Dispute struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	DisputeNo   string    `gorm:"uniqueIndex" json:"dispute_no"`
	WaybillID   uint      `json:"waybill_id"`
	Waybill     Waybill   `json:"waybill"`
	Type        string    `json:"type"` // pieces / damage / rate
	Title       string    `json:"title"`
	Detail      string    `json:"detail"`
	Status      string    `json:"status"` // open / processing / resolved
	Resolution  string    `json:"resolution"`
	CreatedByID uint      `json:"created_by_id"`
	CreatedBy   User      `json:"created_by"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}
