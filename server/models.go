package main

import "time"

// 角色:driver 司机 / ops 运营 / finance 财务 / admin 管理员
type User struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Username  string    `gorm:"uniqueIndex;size:64" json:"username"`
	Password  string    `gorm:"size:128" json:"-"`
	Name      string    `gorm:"size:64" json:"name"`
	Role      string    `gorm:"size:16;index" json:"role"`
	Token     string    `gorm:"uniqueIndex;size:64" json:"-"`
	CreatedAt time.Time `json:"created_at"`
}

// 费率版本:财务调整必须按"原业务发生时间"生效的版本计算
type RateVersion struct {
	ID                      uint      `gorm:"primaryKey" json:"id"`
	Version                 string    `gorm:"uniqueIndex;size:32" json:"version"`
	EffectiveFrom           time.Time `gorm:"index" json:"effective_from"`
	UnitPrice               float64   `json:"unit_price"`                 // 每件运费单价(元/件)
	DamageDeductionPerPiece float64   `json:"damage_deduction_per_piece"` // 每件破损扣款(元/件)
	Remark                  string    `gorm:"size:255" json:"remark"`
	CreatedAt               time.Time `json:"created_at"`
}

// 运单状态
const (
	WaybillPendingReceipt = "pending_receipt" // 待回单
	WaybillReceiptPending = "receipt_pending" // 回单待审核
	WaybillSigned         = "signed"          // 已签收(全部)
	WaybillPartialSigned  = "partial_signed"  // 部分签收
)

type Waybill struct {
	ID             uint      `gorm:"primaryKey" json:"id"`
	WaybillNo      string    `gorm:"uniqueIndex;size:32" json:"waybill_no"`
	DriverID       uint      `gorm:"index" json:"driver_id"`
	Driver         *User     `gorm:"foreignKey:DriverID" json:"driver,omitempty"`
	Customer       string    `gorm:"size:64" json:"customer"`
	Origin         string    `gorm:"size:64" json:"origin"`
	Destination    string    `gorm:"size:64" json:"destination"`
	BusinessDate   time.Time `gorm:"index" json:"business_date"` // 原业务发生时间(费率版本按它取)
	PlannedPieces  int       `json:"planned_pieces"`             // 计划件数
	ContractFreight float64  `json:"contract_freight"`           // 合同运费(原结算金额)
	Status         string    `gorm:"size:32;index" json:"status"`
	CreatedAt      time.Time `json:"created_at"`
}

// 回单状态
const (
	ReceiptPending  = "pending"
	ReceiptApproved = "approved"
	ReceiptRejected = "rejected"
)

// 回单(司机补录的签收证据)
type Receipt struct {
	ID                uint       `gorm:"primaryKey" json:"id"`
	WaybillID         uint       `gorm:"index" json:"waybill_id"`
	UploaderID        uint       `gorm:"index" json:"uploader_id"`
	Uploader          *User      `gorm:"foreignKey:UploaderID" json:"uploader,omitempty"`
	ReceivedPieces    int        `json:"received_pieces"` // 实际签收件数
	DamagedPieces     int        `json:"damaged_pieces"`  // 破损件数
	DamageNote        string     `gorm:"size:255" json:"damage_note"`
	CustomerRefusedSeal bool     `json:"customer_refused_seal"` // 客户拒盖章
	DeclaredSignedAt  time.Time  `json:"declared_signed_at"`    // 司机申报的原签收时刻(仅申报,不作准)
	RecordedAt        time.Time  `json:"recorded_at"`           // 实际补录时间(服务器记录,不可伪造)
	Attachments       string     `gorm:"size:1024" json:"attachments"` // JSON 数组,本地文件路径
	Status            string     `gorm:"size:16;index" json:"status"`
	ReviewerID        *uint      `json:"reviewer_id"`
	Reviewer          *User      `gorm:"foreignKey:ReviewerID" json:"reviewer,omitempty"`
	ReviewedAt        *time.Time `json:"reviewed_at"`
	ReviewNote        string     `gorm:"size:255" json:"review_note"`
	IdempotencyKey    string     `gorm:"uniqueIndex;size:64" json:"idempotency_key"` // 幂等键,防重复提交
	CreatedAt         time.Time  `json:"created_at"`
}

// 结算单:原结算记录,金额一经生成不可改(调整走 Adjustment)
type Settlement struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	WaybillID  uint      `gorm:"uniqueIndex" json:"waybill_id"`
	Amount     float64   `json:"amount"` // 原结算金额(=合同运费)
	Status     string    `gorm:"size:16" json:"status"` // unpaid / paid
	CreatedAt  time.Time `json:"created_at"`
}

// 支付记录(模拟支付接口产生)
type Payment struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	SettlementID uint      `gorm:"index" json:"settlement_id"`
	Amount       float64   `json:"amount"`
	TxnNo        string    `gorm:"size:64" json:"txn_no"`
	OperatorID   uint      `json:"operator_id"`
	Operator     *User     `gorm:"foreignKey:OperatorID" json:"operator,omitempty"`
	PaidAt       time.Time `json:"paid_at"`
}

// 运费调整单:不覆盖原结算,作为独立记录追加;保留费率版本快照与计算依据
type Adjustment struct {
	ID                 uint      `gorm:"primaryKey" json:"id"`
	AdjustNo           string    `gorm:"uniqueIndex;size:32" json:"adjust_no"`
	WaybillID          uint      `gorm:"index" json:"waybill_id"`
	SettlementID       uint      `gorm:"index" json:"settlement_id"`
	ReceiptID          uint      `gorm:"index" json:"receipt_id"`
	RateVersionID      uint      `gorm:"index" json:"rate_version_id"`
	RateVersion        *RateVersion `gorm:"foreignKey:RateVersionID" json:"rate_version,omitempty"`
	RateVersionName    string    `gorm:"size:32" json:"rate_version_name"` // 快照
	ReceivedPieces     int       `json:"received_pieces"`
	DamagedPieces      int       `json:"damaged_pieces"`
	UnitPrice          float64   `json:"unit_price"`           // 快照:费率版本单价
	DamageDeduction    float64   `json:"damage_deduction"`     // 快照:每件破损扣款
	PayableAmount      float64   `json:"payable_amount"`       // 应付 = 签收*单价 - 破损*扣款
	AlreadySettled     float64   `json:"already_settled"`      // 原结算 + 此前调整累计
	DiffAmount         float64   `json:"diff_amount"`          // 调整金额 = 应付 - 已结算
	CalcDetail         string    `gorm:"size:1024" json:"calc_detail"` // 计算依据
	Note               string    `gorm:"size:255" json:"note"`
	CreatorID          uint      `json:"creator_id"`
	Creator            *User     `gorm:"foreignKey:CreatorID" json:"creator,omitempty"`
	CreatedAt          time.Time `json:"created_at"`
}

// 争议类型
const (
	DisputePieces = "pieces" // 件数争议
	DisputeDamage = "damage" // 破损争议
	DisputeRate   = "rate"   // 费率争议
	DisputeOther  = "other"
)

// 争议状态
const (
	DisputeOpen       = "open"
	DisputeProcessing = "processing"
	DisputeResolved   = "resolved"
)

type Dispute struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	DisputeNo    string    `gorm:"uniqueIndex;size:32" json:"dispute_no"`
	WaybillID    uint      `gorm:"index" json:"waybill_id"`
	ReceiptID    *uint     `json:"receipt_id"`
	AdjustmentID *uint     `json:"adjustment_id"`
	Type         string    `gorm:"size:16;index" json:"type"`
	Title        string    `gorm:"size:128" json:"title"`
	Description  string    `gorm:"size:1024" json:"description"`
	Status       string    `gorm:"size:16;index" json:"status"`
	CreatorID    uint      `json:"creator_id"`
	Creator      *User     `gorm:"foreignKey:CreatorID" json:"creator,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
	Handlings    []DisputeHandling `gorm:"foreignKey:DisputeID" json:"handlings,omitempty"`
}

// 争议后续处理记录(部分签收、拒盖章、已付款差额等都留痕)
type DisputeHandling struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	DisputeID  uint      `gorm:"index" json:"dispute_id"`
	Action     string    `gorm:"size:64" json:"action"`
	Note       string    `gorm:"size:1024" json:"note"`
	OperatorID uint      `json:"operator_id"`
	Operator   *User     `gorm:"foreignKey:OperatorID" json:"operator,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
}

// 操作审计日志
type AuditLog struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	WaybillID *uint     `gorm:"index" json:"waybill_id"`
	ActorID   uint      `json:"actor_id"`
	ActorName string    `gorm:"size:64" json:"actor_name"`
	Action    string    `gorm:"size:64" json:"action"`
	Detail    string    `gorm:"size:1024" json:"detail"`
	CreatedAt time.Time `json:"created_at"`
}
