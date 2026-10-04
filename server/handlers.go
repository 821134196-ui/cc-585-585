package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type Handler struct {
	db        *gorm.DB
	uploadDir string
}

// ---------- 登录 ----------

func (h *Handler) login(c *gin.Context) {
	var req struct {
		Username string `json:"username" binding:"required"`
		Password string `json:"password" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请输入用户名和密码"})
		return
	}
	var u User
	if err := h.db.Where("username = ? AND password = ?", req.Username, req.Password).First(&u).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "用户名或密码错误"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"token": u.Token, "user": u})
}

// 演示用:公开可查看的账号列表
func (h *Handler) accounts(c *gin.Context) {
	var users []User
	h.db.Order("id").Find(&users)
	type acc struct {
		Username string `json:"username"`
		Password string `json:"password"`
		Name     string `json:"name"`
		Role     string `json:"role"`
		RoleName string `json:"role_name"`
	}
	out := make([]acc, 0, len(users))
	for _, u := range users {
		out = append(out, acc{u.Username, u.Password, u.Name, u.Role, roleName(u.Role)})
	}
	c.JSON(http.StatusOK, out)
}

func (h *Handler) me(c *gin.Context) {
	c.JSON(http.StatusOK, currentUser(c))
}

// ---------- 运单 ----------

func (h *Handler) listWaybills(c *gin.Context) {
	u := currentUser(c)
	q := h.db.Model(&Waybill{}).Preload("Driver").Order("id")
	if s := c.Query("status"); s != "" {
		q = q.Where("status = ?", s)
	}
	if kw := c.Query("q"); kw != "" {
		q = q.Where("waybill_no LIKE ? OR customer LIKE ?", "%"+kw+"%", "%"+kw+"%")
	}
	if c.Query("mine") == "1" {
		q = q.Where("driver_id = ?", u.ID)
	}
	var wbs []Waybill
	if err := q.Find(&wbs).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	// 汇总每张运单的最新回单/结算状态
	type item struct {
		Waybill
		StatusName      string   `json:"status_name"`
		ReceivedPieces  *int     `json:"received_pieces"`
		DamagedPieces   *int     `json:"damaged_pieces"`
		SettlementStatus string  `json:"settlement_status"`
		ReceiptStatus   string   `json:"receipt_status"`
	}
	out := make([]item, 0, len(wbs))
	for _, wb := range wbs {
		it := item{Waybill: wb, StatusName: waybillStatusName(wb.Status)}
		var r Receipt
		if err := h.db.Where("waybill_id = ?", wb.ID).Order("id DESC").First(&r).Error; err == nil {
			it.ReceivedPieces = &r.ReceivedPieces
			it.DamagedPieces = &r.DamagedPieces
			it.ReceiptStatus = r.Status
		}
		var st Settlement
		if err := h.db.Where("waybill_id = ?", wb.ID).First(&st).Error; err == nil {
			it.SettlementStatus = st.Status
		}
		out = append(out, it)
	}
	c.JSON(http.StatusOK, out)
}

func (h *Handler) waybillDetail(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var wb Waybill
	if err := h.db.Preload("Driver").First(&wb, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "运单不存在"})
		return
	}
	var receipts []Receipt
	h.db.Preload("Uploader").Preload("Reviewer").Where("waybill_id = ?", wb.ID).Order("id DESC").Find(&receipts)
	var settlement Settlement
	stFound := h.db.Where("waybill_id = ?", wb.ID).First(&settlement).Error == nil
	var payments []Payment
	if stFound {
		h.db.Preload("Operator").Where("settlement_id = ?", settlement.ID).Order("id").Find(&payments)
	}
	var adjustments []Adjustment
	h.db.Preload("Creator").Preload("RateVersion").Where("waybill_id = ?", wb.ID).Order("id").Find(&adjustments)
	var disputes []Dispute
	h.db.Preload("Creator").Preload("Handlings.Operator").Where("waybill_id = ?", wb.ID).Order("id DESC").Find(&disputes)
	var audits []AuditLog
	h.db.Where("waybill_id = ?", wb.ID).Order("id DESC").Limit(50).Find(&audits)

	// 该运单业务时间适用的费率版本(供财务参考)
	var applicableRate *RateVersion
	if rv, err := resolveRateVersion(h.db, wb.BusinessDate); err == nil {
		applicableRate = rv
	}

	c.JSON(http.StatusOK, gin.H{
		"waybill":          wb,
		"status_name":      waybillStatusName(wb.Status),
		"receipts":         receipts,
		"settlement":       stFoundOrNil(stFound, &settlement),
		"payments":         payments,
		"adjustments":      adjustments,
		"disputes":         disputes,
		"audits":           audits,
		"applicable_rate":  applicableRate,
	})
}

func stFoundOrNil(found bool, st *Settlement) interface{} {
	if !found {
		return nil
	}
	return st
}

// ---------- 回单补录 ----------

// 司机补录回单:multipart 表单,支持附件;幂等键防重复提交
func (h *Handler) submitReceipt(c *gin.Context) {
	u := currentUser(c)
	wbID, _ := strconv.Atoi(c.Param("id"))
	var wb Waybill
	if err := h.db.First(&wb, wbID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "运单不存在"})
		return
	}
	if u.Role == "driver" && wb.DriverID != u.ID {
		c.JSON(http.StatusForbidden, gin.H{"error": "只能补录自己承运的运单"})
		return
	}

	received, err1 := strconv.Atoi(c.PostForm("received_pieces"))
	damaged, err2 := strconv.Atoi(c.PostForm("damaged_pieces"))
	declaredStr := c.PostForm("declared_signed_at")
	idemKey := strings.TrimSpace(c.PostForm("idempotency_key"))
	if err1 != nil || err2 != nil || declaredStr == "" || idemKey == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少必填字段:received_pieces / damaged_pieces / declared_signed_at / idempotency_key"})
		return
	}

	// 幂等:同一 idempotency_key 重复提交直接返回原回单(双击/重试不产生第二条)
	var existing Receipt
	if err := h.db.Where("idempotency_key = ?", idemKey).First(&existing).Error; err == nil {
		c.JSON(http.StatusOK, gin.H{"receipt": existing, "idempotent_replay": true, "message": "重复提交已被幂等处理,返回原回单"})
		return
	}

	// 件数校验
	if received < 0 || received > wb.PlannedPieces {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("实际签收件数须在 0~%d(计划件数)之间", wb.PlannedPieces)})
		return
	}
	if damaged < 0 || damaged > received {
		c.JSON(http.StatusBadRequest, gin.H{"error": "破损件数不能超过实际签收件数"})
		return
	}

	// 时间防伪造:补录时间由服务器记录;申报签收时刻不得晚于当前、不得早于业务发生日
	declared, err := time.ParseInLocation("2006-01-02T15:04", declaredStr, time.Local)
	if err != nil {
		declared, err = time.ParseInLocation("2006-01-02 15:04:05", declaredStr, time.Local)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "declared_signed_at 格式应为 2006-01-02T15:04"})
			return
		}
	}
	now := time.Now()
	if declared.After(now) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "申报的签收时间晚于当前时间,疑似伪造,已拒绝"})
		return
	}
	if declared.Before(wb.BusinessDate) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "申报的签收时间早于业务发生时间,不合理"})
		return
	}

	// 重复回单拦截:已有待审或已审回单,不允许第二次签收
	var cnt int64
	h.db.Model(&Receipt{}).Where("waybill_id = ? AND status IN ?", wb.ID, []string{ReceiptPending, ReceiptApproved}).Count(&cnt)
	if cnt > 0 {
		c.JSON(http.StatusConflict, gin.H{"error": "该运单已存在回单(待审核或已生效),不能重复补录生成第二次签收"})
		return
	}

	// 附件存本地
	attachments := []string{}
	if form, err := c.MultipartForm(); err == nil {
		for _, fh := range form.File["files"] {
			if fh.Size > 10<<20 {
				c.JSON(http.StatusBadRequest, gin.H{"error": "单个附件不能超过 10MB"})
				return
			}
			name := fmt.Sprintf("%d_%s", now.UnixNano(), filepath.Base(fh.Filename))
			dst := filepath.Join(h.uploadDir, name)
			if err := c.SaveUploadedFile(fh, dst); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "附件保存失败:" + err.Error()})
				return
			}
			attachments = append(attachments, "/uploads/"+name)
		}
	}
	attJSON, _ := json.Marshal(attachments)

	refused := c.PostForm("customer_refused_seal") == "true"
	receipt := Receipt{
		WaybillID: wb.ID, UploaderID: u.ID,
		ReceivedPieces: received, DamagedPieces: damaged,
		DamageNote: c.PostForm("damage_note"), CustomerRefusedSeal: refused,
		DeclaredSignedAt: declared, RecordedAt: now, // 补录时间以服务器为准
		Status: ReceiptPending, IdempotencyKey: idemKey,
		Attachments: string(attJSON), CreatedAt: now,
	}
	if err := h.db.Create(&receipt).Error; err != nil {
		if strings.Contains(err.Error(), "idempotency") || strings.Contains(err.Error(), "UNIQUE") {
			c.JSON(http.StatusConflict, gin.H{"error": "重复提交已被拦截"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	refreshWaybillStatus(h.db, wb.ID)
	writeAudit(h.db, &wb.ID, u, "提交回单补录",
		fmt.Sprintf("实收 %d/%d 件,破损 %d 件;申报签收 %s,系统补录时间 %s",
			received, wb.PlannedPieces, damaged, declared.Format("01-02 15:04"), now.Format("01-02 15:04")))

	// 部分签收/拒盖章自动生成争议线索,便于运营跟踪
	if received < wb.PlannedPieces {
		h.createSystemDispute(wb, &receipt, DisputePieces, "部分签收",
			fmt.Sprintf("计划 %d 件,实际签收 %d 件,差 %d 件。", wb.PlannedPieces, received, wb.PlannedPieces-received), u)
	}
	if damaged > 0 {
		h.createSystemDispute(wb, &receipt, DisputeDamage, "货物破损",
			fmt.Sprintf("破损 %d 件。%s", damaged, c.PostForm("damage_note")), u)
	}
	if refused {
		h.createSystemDispute(wb, &receipt, DisputeOther, "客户拒盖章",
			"客户拒绝在回单上盖章,需运营线下核实签收事实。", u)
	}

	h.db.Preload("Uploader").First(&receipt, receipt.ID)
	c.JSON(http.StatusCreated, gin.H{"receipt": receipt, "message": "回单已提交,等待运营审核"})
}

func (h *Handler) createSystemDispute(wb Waybill, r *Receipt, typ, title, desc string, u *User) {
	no := fmt.Sprintf("DP%s%06d", time.Now().Format("20060102"), time.Now().UnixNano()%1000000)
	d := Dispute{
		DisputeNo: no, WaybillID: wb.ID, ReceiptID: &r.ID, Type: typ,
		Title: title, Description: desc, Status: DisputeOpen,
		CreatorID: u.ID, CreatedAt: time.Now(),
	}
	h.db.Create(&d)
}

// ---------- 回单审核 ----------

func (h *Handler) reviewReceipt(c *gin.Context) {
	u := currentUser(c)
	id, _ := strconv.Atoi(c.Param("id"))
	var r Receipt
	if err := h.db.First(&r, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "回单不存在"})
		return
	}
	// 关键规则:司机不能审核自己的补录(任何角色都不能审核自己提交的回单)
	if r.UploaderID == u.ID {
		c.JSON(http.StatusForbidden, gin.H{"error": "不能审核自己提交的回单补录,需由其他运营人员审核"})
		return
	}
	if r.Status != ReceiptPending {
		c.JSON(http.StatusConflict, gin.H{"error": "该回单已审核过,不能重复审核"})
		return
	}
	var req struct {
		Approve bool   `json:"approve"`
		Note    string `json:"note"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}
	now := time.Now()
	r.ReviewerID = &u.ID
	r.ReviewedAt = &now
	r.ReviewNote = req.Note
	if req.Approve {
		r.Status = ReceiptApproved
	} else {
		r.Status = ReceiptRejected
	}
	if err := h.db.Save(&r).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	refreshWaybillStatus(h.db, r.WaybillID)
	action := "审核通过回单"
	if !req.Approve {
		action = "驳回回单"
	}
	writeAudit(h.db, &r.WaybillID, u, action, fmt.Sprintf("回单 #%d:%s", r.ID, req.Note))
	c.JSON(http.StatusOK, gin.H{"receipt": r, "message": action + "成功"})
}

// ---------- 费率版本 ----------

func (h *Handler) listRateVersions(c *gin.Context) {
	var rvs []RateVersion
	h.db.Order("effective_from").Find(&rvs)
	c.JSON(http.StatusOK, rvs)
}

// 查询某业务时间适用的费率版本
func (h *Handler) resolveRate(c *gin.Context) {
	atStr := c.Query("at")
	at, err := time.ParseInLocation("2006-01-02", atStr, time.Local)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "at 参数格式应为 2006-01-02"})
		return
	}
	rv, err := resolveRateVersion(h.db, at)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, rv)
}

// ---------- 运费调整单 ----------

func (h *Handler) createAdjustment(c *gin.Context) {
	u := currentUser(c)
	wbID, _ := strconv.Atoi(c.Param("id"))
	var wb Waybill
	if err := h.db.First(&wb, wbID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "运单不存在"})
		return
	}
	// 必须有已审核回单
	var r Receipt
	if err := h.db.Where("waybill_id = ? AND status = ?", wb.ID, ReceiptApproved).Order("id DESC").First(&r).Error; err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "该运单没有已审核回单,不能生成调整单"})
		return
	}
	var st Settlement
	if err := h.db.Where("waybill_id = ?", wb.ID).First(&st).Error; err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "该运单没有原结算记录"})
		return
	}
	// 按原业务发生时间取费率版本
	rv, err := resolveRateVersion(h.db, wb.BusinessDate)
	if err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}

	var req struct {
		Note            string   `json:"note"`
		ExpectedPayable *float64 `json:"expected_payable"` // 可选:前端预算值,用于双方核对
		OverrideReceived *int    `json:"override_received_pieces"` // 可选:付款后发现差额等场景,以争议核实件数为准
		OverrideDamaged  *int    `json:"override_damaged_pieces"`
	}
	c.ShouldBindJSON(&req)

	received := r.ReceivedPieces
	damaged := r.DamagedPieces
	overrideNote := ""
	if req.OverrideReceived != nil && (*req.OverrideReceived != r.ReceivedPieces || (req.OverrideDamaged != nil && *req.OverrideDamaged != r.DamagedPieces)) {
		if *req.OverrideReceived < 0 || *req.OverrideReceived > wb.PlannedPieces {
			c.JSON(http.StatusBadRequest, gin.H{"error": "核实件数须在 0~计划件数之间"})
			return
		}
		if req.Note == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "按争议核实件数调整时必须填写备注,说明依据(如对应争议单号)"})
			return
		}
		received = *req.OverrideReceived
		if req.OverrideDamaged != nil {
			damaged = *req.OverrideDamaged
		}
		overrideNote = fmt.Sprintf(";件数以争议核实为准(回单原记录实收 %d 件、破损 %d 件)", r.ReceivedPieces, r.DamagedPieces)
	}
	if damaged < 0 || damaged > received {
		c.JSON(http.StatusBadRequest, gin.H{"error": "破损件数不能超过签收(核实)件数"})
		return
	}

	payable := round2(float64(received)*rv.UnitPrice - float64(damaged)*rv.DamageDeductionPerPiece)
	if req.ExpectedPayable != nil && round2(*req.ExpectedPayable) != payable {
		c.JSON(http.StatusUnprocessableEntity, gin.H{
			"error": fmt.Sprintf("金额核对不一致:系统按费率版本 %s 计算应付 %.2f,与提交的预期 %.2f 不符,已拒绝", rv.Version, payable, *req.ExpectedPayable),
			"computed_payable": payable,
		})
		return
	}

	// 已结算 = 原结算 + 历史调整累计(不覆盖原结算,调整逐笔追加)
	var prevSum float64
	h.db.Model(&Adjustment{}).Where("waybill_id = ?", wb.ID).Select("COALESCE(SUM(diff_amount),0)").Scan(&prevSum)
	already := round2(st.Amount + prevSum)
	diff := round2(payable - already)

	calcDetail := fmt.Sprintf(
		"费率版本 %s(自 %s 生效,业务发生 %s):应付 = 实收 %d 件 × %.2f 元/件 - 破损 %d 件 × %.2f 元/件 = %.2f 元;已结算 = 原结算 %.2f + 历史调整 %.2f = %.2f 元;本次调整 = %.2f - %.2f = %.2f 元",
		rv.Version, rv.EffectiveFrom.Format("2006-01-02"), wb.BusinessDate.Format("2006-01-02"),
		received, rv.UnitPrice, damaged, rv.DamageDeductionPerPiece, payable,
		st.Amount, prevSum, already, payable, already, diff)
	calcDetail += overrideNote
	if st.Status == "paid" {
		calcDetail += ";注意:原结算已付款,本调整为付款后差额补记,原结算记录保持不变"
	}

	adj := Adjustment{
		AdjustNo: fmt.Sprintf("ADJ%s%06d", time.Now().Format("20060102"), time.Now().UnixNano()%1000000),
		WaybillID: wb.ID, SettlementID: st.ID, ReceiptID: r.ID,
		RateVersionID: rv.ID, RateVersionName: rv.Version,
		ReceivedPieces: received, DamagedPieces: damaged,
		UnitPrice: rv.UnitPrice, DamageDeduction: rv.DamageDeductionPerPiece,
		PayableAmount: payable, AlreadySettled: already, DiffAmount: diff,
		CalcDetail: calcDetail, Note: req.Note,
		CreatorID: u.ID, CreatedAt: time.Now(),
	}
	if err := h.db.Create(&adj).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	writeAudit(h.db, &wb.ID, u, "生成运费调整单",
		fmt.Sprintf("%s:调整 %.2f 元(费率版本 %s)", adj.AdjustNo, diff, rv.Version))
	h.db.Preload("Creator").Preload("RateVersion").First(&adj, adj.ID)
	c.JSON(http.StatusCreated, gin.H{"adjustment": adj, "message": "调整单已生成,原结算记录未改动"})
}

// ---------- 模拟支付 ----------

func (h *Handler) paySettlement(c *gin.Context) {
	u := currentUser(c)
	id, _ := strconv.Atoi(c.Param("id"))
	var st Settlement
	if err := h.db.First(&st, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "结算单不存在"})
		return
	}
	if st.Status == "paid" {
		c.JSON(http.StatusConflict, gin.H{"error": "该结算单已付款,不能重复支付"})
		return
	}
	// 本地模拟支付网关
	txn := mockPayGateway(st.Amount)
	now := time.Now()
	pay := Payment{SettlementID: st.ID, Amount: st.Amount, TxnNo: txn, OperatorID: u.ID, PaidAt: now}
	if err := h.db.Create(&pay).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	h.db.Model(&st).Update("status", "paid")
	writeAudit(h.db, &st.WaybillID, u, "结算付款", fmt.Sprintf("金额 %.2f 元,模拟交易号 %s", st.Amount, txn))
	c.JSON(http.StatusOK, gin.H{"payment": pay, "message": "支付成功(本地模拟)"})
}

// 模拟支付网关:返回交易号
func mockPayGateway(amount float64) string {
	return fmt.Sprintf("PAY%s%06d", time.Now().Format("20060102"), time.Now().UnixNano()%1000000)
}

// ---------- 模拟扫描接口 ----------

// 模拟扫描仪/OCR:根据运单号返回识别出的回单字段,用于预填补录表单
func (h *Handler) scanMock(c *gin.Context) {
	var req struct {
		WaybillNo string `json:"waybill_no" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请提供 waybill_no"})
		return
	}
	var wb Waybill
	if err := h.db.Where("waybill_no = ?", req.WaybillNo).First(&wb).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "扫描失败:未找到运单 " + req.WaybillNo})
		return
	}
	// 模拟 OCR 识别结果(本地模拟,确定性输出)
	received := wb.PlannedPieces
	damaged := 0
	if wb.WaybillNo == "WB20250310001" { // 演示部分签收+破损
		received = 95
		damaged = 3
	}
	c.JSON(http.StatusOK, gin.H{
		"waybill_no":         wb.WaybillNo,
		"received_pieces":    received,
		"damaged_pieces":     damaged,
		"declared_signed_at": wb.BusinessDate.Add(26 * time.Hour).Format("2006-01-02T15:04"),
		"confidence":         0.98,
		"message":            "扫描识别成功(本地模拟),请核对后提交",
	})
}

// ---------- 争议 ----------

func (h *Handler) createDispute(c *gin.Context) {
	u := currentUser(c)
	wbID, _ := strconv.Atoi(c.Param("id"))
	var wb Waybill
	if err := h.db.First(&wb, wbID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "运单不存在"})
		return
	}
	var req struct {
		Type        string `json:"type" binding:"required"`
		Title       string `json:"title" binding:"required"`
		Description string `json:"description"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请填写争议类型和标题"})
		return
	}
	switch req.Type {
	case DisputePieces, DisputeDamage, DisputeRate, DisputeOther:
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "争议类型须为 pieces/damage/rate/other"})
		return
	}
	d := Dispute{
		DisputeNo: fmt.Sprintf("DP%s%06d", time.Now().Format("20060102"), time.Now().UnixNano()%1000000),
		WaybillID: wb.ID, Type: req.Type, Title: req.Title, Description: req.Description,
		Status: DisputeOpen, CreatorID: u.ID, CreatedAt: time.Now(),
	}
	if err := h.db.Create(&d).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	writeAudit(h.db, &wb.ID, u, "发起争议", fmt.Sprintf("%s [%s] %s", d.DisputeNo, req.Type, req.Title))
	c.JSON(http.StatusCreated, gin.H{"dispute": d, "message": "争议已登记"})
}

func (h *Handler) addDisputeHandling(c *gin.Context) {
	u := currentUser(c)
	id, _ := strconv.Atoi(c.Param("id"))
	var d Dispute
	if err := h.db.First(&d, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "争议不存在"})
		return
	}
	var req struct {
		Action string `json:"action" binding:"required"`
		Note   string `json:"note"`
		Status string `json:"status"` // 可选:同步更新争议状态
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请填写处理动作"})
		return
	}
	hg := DisputeHandling{DisputeID: d.ID, Action: req.Action, Note: req.Note, OperatorID: u.ID, CreatedAt: time.Now()}
	if err := h.db.Create(&hg).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if req.Status != "" {
		switch req.Status {
		case DisputeOpen, DisputeProcessing, DisputeResolved:
			h.db.Model(&d).Update("status", req.Status)
		}
	}
	writeAudit(h.db, &d.WaybillID, u, "处理争议", fmt.Sprintf("%s:%s", d.DisputeNo, req.Action))
	c.JSON(http.StatusCreated, gin.H{"handling": hg, "message": "处理记录已保存"})
}

// ---------- 审计日志 ----------

func (h *Handler) listAudits(c *gin.Context) {
	q := h.db.Order("id DESC").Limit(100)
	if wid := c.Query("waybill_id"); wid != "" {
		q = q.Where("waybill_id = ?", wid)
	}
	var logs []AuditLog
	q.Find(&logs)
	c.JSON(http.StatusOK, logs)
}

// 确保上传目录存在
func ensureDir(dir string) {
	os.MkdirAll(dir, 0o755)
}
