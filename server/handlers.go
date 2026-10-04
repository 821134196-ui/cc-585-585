package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func round2(f float64) float64 { return math.Round(f*100) / 100 }

// ---------- 登录 ----------

func loginHandler(c *gin.Context) {
	var req struct {
		Username string `json:"username"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Username == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请提供用户名"})
		return
	}
	var u User
	if err := db.Where("username = ?", req.Username).First(&u).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "用户不存在,可选: driver1 / driver2 / ops1 / finance1"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"token": newToken(u.ID), "user": u})
}

func meHandler(c *gin.Context) {
	c.JSON(http.StatusOK, currentUser(c))
}

// ---------- 运单 ----------

func listWaybills(c *gin.Context) {
	q := db.Preload("Driver").Preload("Settlement").Preload("Settlement.RateVersion").Order("id desc")
	if s := c.Query("status"); s != "" {
		q = q.Where("status = ?", s)
	}
	if kw := c.Query("q"); kw != "" {
		q = q.Where("waybill_no LIKE ? OR customer_name LIKE ?", "%"+kw+"%", "%"+kw+"%")
	}
	u := currentUser(c)
	if u.Role == "driver" { // 司机只看自己的运单
		q = q.Where("driver_id = ?", u.ID)
	}
	var list []Waybill
	q.Find(&list)
	// 补充每单最新回单状态
	type Item struct {
		Waybill
		PendingReceipts int `json:"pending_receipts"`
	}
	items := make([]Item, 0, len(list))
	for _, w := range list {
		var n int64
		db.Model(&Receipt{}).Where("waybill_id = ? AND status = 'pending'", w.ID).Count(&n)
		items = append(items, Item{w, int(n)})
	}
	c.JSON(http.StatusOK, items)
}

func getWaybill(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var w Waybill
	if err := db.Preload("Driver").Preload("Settlement").Preload("Settlement.RateVersion").
		Preload("Receipts", func(db *gorm.DB) *gorm.DB { return db.Order("receipts.id desc") }).
		Preload("Receipts.Uploader").Preload("Receipts.Reviewer").Preload("Receipts.Attachments").
		First(&w, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "运单不存在"})
		return
	}
	var adjustments []Adjustment
	db.Preload("RateVersion").Preload("CreatedBy").Where("waybill_id = ?", w.ID).Order("id desc").Find(&adjustments)
	var disputes []Dispute
	db.Preload("CreatedBy").Where("waybill_id = ?", w.ID).Order("id desc").Find(&disputes)
	c.JSON(http.StatusOK, gin.H{
		"waybill": w, "adjustments": adjustments, "disputes": disputes,
	})
}

// ---------- 回单补录(司机) ----------

func parseTimeFlexible(s string) (time.Time, error) {
	for _, layout := range []string{time.RFC3339, "2006-01-02T15:04", "2006-01-02 15:04:05", "2006-01-02"} {
		if t, err := time.ParseInLocation(layout, s, time.Local); err == nil {
			return t, nil
		}
	}
	return time.Time{}, errors.New("时间格式不正确")
}

func uploadReceipt(c *gin.Context) {
	u := currentUser(c)
	id, _ := strconv.Atoi(c.Param("id"))
	var w Waybill
	if err := db.First(&w, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "运单不存在"})
		return
	}
	if w.DriverID != u.ID {
		c.JSON(http.StatusForbidden, gin.H{"error": "只能补录自己承运的运单"})
		return
	}
	// 规则: 已有审核通过的回单 => 不能重复补录生成第二次签收
	var approvedCnt int64
	db.Model(&Receipt{}).Where("waybill_id = ? AND status = 'approved'", w.ID).Count(&approvedCnt)
	if approvedCnt > 0 {
		c.JSON(http.StatusConflict, gin.H{"error": "该运单已完成签收,重复回单不能生成第二次签收"})
		return
	}
	var pendingCnt int64
	db.Model(&Receipt{}).Where("waybill_id = ? AND status = 'pending'", w.ID).Count(&pendingCnt)
	if pendingCnt > 0 {
		c.JSON(http.StatusConflict, gin.H{"error": "该运单已有待审核的回单,请等待运营审核"})
		return
	}

	signedPieces, err1 := strconv.Atoi(c.PostForm("signed_pieces"))
	damagedPieces, err2 := strconv.Atoi(c.PostForm("damaged_pieces"))
	if err1 != nil || err2 != nil || signedPieces < 0 || damagedPieces < 0 || damagedPieces > signedPieces {
		c.JSON(http.StatusBadRequest, gin.H{"error": "签收/破损件数不合法(破损不能大于签收)"})
		return
	}
	if signedPieces > w.PlannedPieces {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("签收件数(%d)不能超过计划件数(%d)", signedPieces, w.PlannedPieces)})
		return
	}
	claimed, err := parseTimeFlexible(c.PostForm("claimed_sign_time"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "签收时间格式不正确"})
		return
	}
	stamp := c.PostForm("stamp_status")
	if stamp != "stamped" && stamp != "refused" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "盖章状态必须为 stamped 或 refused"})
		return
	}

	form, err := c.MultipartForm()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "表单解析失败"})
		return
	}
	files := form.File["files"]
	if len(files) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请至少上传一张回单照片"})
		return
	}

	rc := Receipt{
		WaybillID:       w.ID,
		UploaderID:      u.ID,
		ClaimedSignTime: claimed,
		EntryTime:       time.Now(), // 实际补录时间由服务器盖章,客户端无法伪造
		SignedPieces:    signedPieces,
		DamagedPieces:   damagedPieces,
		StampStatus:     stamp,
		Note:            c.PostForm("note"),
		Status:          "pending",
	}

	err = db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&rc).Error; err != nil {
			return err
		}
		for _, f := range files {
			if f.Size > 10<<20 {
				return errors.New("单个附件不能超过 10MB")
			}
			ext := strings.ToLower(filepath.Ext(f.Filename))
			stored := fmt.Sprintf("receipt_%d_%d%s", rc.ID, time.Now().UnixNano(), ext)
			dst := filepath.Join(envOr("UPLOAD_DIR", "../uploads"), stored)
			if err := c.SaveUploadedFile(f, dst); err != nil {
				return err
			}
			att := Attachment{ReceiptID: rc.ID, FileName: f.Filename, StoredName: stored, ContentType: f.Header.Get("Content-Type"), Size: f.Size}
			if err := tx.Create(&att).Error; err != nil {
				return err
			}
		}
		return tx.Model(&Waybill{}).Where("id = ?", w.ID).Update("status", "receipt_pending").Error
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "保存失败: " + err.Error()})
		return
	}
	db.Preload("Attachments").First(&rc, rc.ID)
	c.JSON(http.StatusOK, gin.H{"receipt": rc, "message": "补录已提交,录入时间以服务器时间为准: " + rc.EntryTime.Format("2006-01-02 15:04:05")})
}

// ---------- 回单审核(运营) ----------

func reviewReceipt(c *gin.Context) {
	u := currentUser(c)
	id, _ := strconv.Atoi(c.Param("id"))
	var req struct {
		Action string `json:"action"` // approve / reject
		Note   string `json:"note"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || (req.Action != "approve" && req.Action != "reject") {
		c.JSON(http.StatusBadRequest, gin.H{"error": "action 必须为 approve 或 reject"})
		return
	}
	var rc Receipt
	if err := db.Preload("Uploader").First(&rc, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "回单不存在"})
		return
	}
	// 规则: 司机不能审核自己的补录(即使拥有运营权限的账号也不允许审自己上传的)
	if rc.UploaderID == u.ID {
		c.JSON(http.StatusForbidden, gin.H{"error": "不能审核自己上传的补录,需由其他运营人员核验"})
		return
	}
	if rc.Status != "pending" {
		c.JSON(http.StatusConflict, gin.H{"error": "该回单已审核,不能重复操作"})
		return
	}

	now := time.Now()
	if req.Action == "reject" {
		db.Model(&rc).Updates(map[string]interface{}{
			"status": "rejected", "reviewer_id": u.ID, "reviewed_at": &now, "review_note": req.Note,
		})
		db.Model(&Waybill{}).Where("id = ?", rc.WaybillID).Update("status", "in_transit")
		c.JSON(http.StatusOK, gin.H{"message": "已驳回"})
		return
	}

	// 审核通过: 双保险检查重复签收
	err := db.Transaction(func(tx *gorm.DB) error {
		var approvedCnt int64
		tx.Model(&Receipt{}).Where("waybill_id = ? AND status = 'approved' AND id <> ?", rc.WaybillID, rc.ID).Count(&approvedCnt)
		if approvedCnt > 0 {
			return errors.New("该运单已有审核通过的回单,重复回单不能生成第二次签收")
		}
		if err := tx.Model(&rc).Updates(map[string]interface{}{
			"status": "approved", "reviewer_id": u.ID, "reviewed_at": &now, "review_note": req.Note,
		}).Error; err != nil {
			return err
		}
		// 签收事实只在这里写入一次
		if err := tx.Model(&Waybill{}).Where("id = ?", rc.WaybillID).Updates(map[string]interface{}{
			"actual_pieces": rc.SignedPieces, "damaged_pieces": rc.DamagedPieces, "status": "signed",
		}).Error; err != nil {
			return err
		}
		// 自动登记争议: 部分签收 / 客户拒盖章 / 破损
		var w Waybill
		tx.First(&w, rc.WaybillID)
		autoDisputes(tx, &w, &rc, u)
		return nil
	})
	if err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "审核通过,签收件数已登记"})
}

func autoDisputes(tx *gorm.DB, w *Waybill, rc *Receipt, u *User) {
	mk := func(typ, title, detail string) {
		d := Dispute{
			DisputeNo: fmt.Sprintf("DSP-%d-%d", time.Now().UnixNano()/1e6, rc.ID),
			WaybillID: w.ID, Type: typ, Title: title, Detail: detail,
			Status: "open", CreatedByID: u.ID,
		}
		tx.Create(&d)
	}
	if rc.SignedPieces < w.PlannedPieces {
		mk("pieces", "部分签收",
			fmt.Sprintf("计划 %d 件,实际签收 %d 件,短少 %d 件,待与客户核实。", w.PlannedPieces, rc.SignedPieces, w.PlannedPieces-rc.SignedPieces))
	}
	if rc.DamagedPieces > 0 {
		mk("damage", "货物破损",
			fmt.Sprintf("签收 %d 件中破损 %d 件,涉及运费扣减与理赔。", rc.SignedPieces, rc.DamagedPieces))
	}
	if rc.StampStatus == "refused" {
		mk("pieces", "客户拒绝盖章",
			"客户拒在回单上盖章,签收事实以照片与补录记录为准,待运营与客户确认。")
	}
}

// ---------- 调整单(财务) ----------

// 按原业务发生时间匹配费率版本
func rateVersionAt(t time.Time) (*RateVersion, error) {
	var rv RateVersion
	err := db.Where("effective_from <= ? AND (effective_to IS NULL OR effective_to >= ?)", t, t).
		Order("effective_from desc").First(&rv).Error
	if err != nil {
		return nil, errors.New("找不到业务发生时间(" + t.Format("2006-01-02") + ")对应的费率版本")
	}
	return &rv, nil
}

func createAdjustment(c *gin.Context) {
	u := currentUser(c)
	id, _ := strconv.Atoi(c.Param("id"))
	var w Waybill
	if err := db.Preload("Settlement").First(&w, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "运单不存在"})
		return
	}
	if w.Settlement == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "该运单尚无原始结算,无法调整"})
		return
	}
	// 必须基于审核通过的回单
	var rc Receipt
	if err := db.Where("waybill_id = ? AND status = 'approved'", w.ID).Order("id desc").First(&rc).Error; err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "需先由运营审核通过回单,才能生成调整单"})
		return
	}
	var existCnt int64
	db.Model(&Adjustment{}).Where("receipt_id = ?", rc.ID).Count(&existCnt)
	if existCnt > 0 {
		c.JSON(http.StatusConflict, gin.H{"error": "该回单已生成过调整单,不能重复生成"})
		return
	}

	// 费率版本按原业务发生时间取,不用当前时间
	rv, err := rateVersionAt(w.BusinessDate)
	if err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}

	adjusted := round2(float64(rc.SignedPieces)*rv.UnitPrice - float64(rc.DamagedPieces)*rv.DamageDeduction)
	original := w.Settlement.Amount
	diff := round2(adjusted - original)

	basis, _ := json.Marshal(gin.H{
		"formula":          "签收件数×单价 − 破损件数×破损扣减",
		"signed_pieces":    rc.SignedPieces,
		"damaged_pieces":   rc.DamagedPieces,
		"unit_price":       rv.UnitPrice,
		"damage_deduction": rv.DamageDeduction,
		"rate_version":     rv.Version,
		"business_date":    w.BusinessDate.Format("2006-01-02"),
		"original_amount":  original,
		"adjusted_amount":  adjusted,
		"difference":       diff,
		"receipt_id":       rc.ID,
		"receipt_entry_at": rc.EntryTime.Format("2006-01-02 15:04:05"),
	})

	adj := Adjustment{
		AdjustNo:        fmt.Sprintf("ADJ-%s", time.Now().Format("20060102150405")),
		WaybillID:       w.ID,
		ReceiptID:       rc.ID,
		SettlementID:    w.Settlement.ID,
		RateVersionID:   rv.ID,
		SignedPieces:    rc.SignedPieces,
		DamagedPieces:   rc.DamagedPieces,
		UnitPrice:       rv.UnitPrice,
		DamageDeduction: rv.DamageDeduction,
		AdjustedAmount:  adjusted,
		OriginalAmount:  original,
		Difference:      diff,
		Basis:           string(basis),
		Status:          "confirmed",
		CreatedByID:     u.ID,
	}
	err = db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&adj).Error; err != nil {
			return err
		}
		// 原结算不动,只把运单状态标记为已调整
		return tx.Model(&Waybill{}).Where("id = ?", w.ID).Update("status", "adjusted").Error
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "生成调整单失败: " + err.Error()})
		return
	}
	db.Preload("RateVersion").Preload("CreatedBy").First(&adj, adj.ID)
	c.JSON(http.StatusOK, gin.H{"adjustment": adj, "message": fmt.Sprintf("调整单已生成,使用费率版本 %s(业务发生日 %s)", rv.Version, w.BusinessDate.Format("2006-01-02"))})
}

func listAdjustments(c *gin.Context) {
	var list []Adjustment
	db.Preload("Waybill").Preload("RateVersion").Preload("CreatedBy").Order("id desc").Find(&list)
	c.JSON(http.StatusOK, list)
}

func listRateVersions(c *gin.Context) {
	var list []RateVersion
	db.Order("effective_from desc").Find(&list)
	c.JSON(http.StatusOK, list)
}

// ---------- 争议 ----------

func listDisputes(c *gin.Context) {
	q := db.Preload("Waybill").Preload("CreatedBy").Order("id desc")
	if s := c.Query("status"); s != "" {
		q = q.Where("status = ?", s)
	}
	var list []Dispute
	q.Find(&list)
	c.JSON(http.StatusOK, list)
}

func createDispute(c *gin.Context) {
	u := currentUser(c)
	id, _ := strconv.Atoi(c.Param("id"))
	var w Waybill
	if err := db.First(&w, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "运单不存在"})
		return
	}
	var req struct {
		Type   string `json:"type"`
		Title  string `json:"title"`
		Detail string `json:"detail"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Title == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请填写争议标题"})
		return
	}
	if req.Type != "pieces" && req.Type != "damage" && req.Type != "rate" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "争议类型必须为 pieces/damage/rate"})
		return
	}
	d := Dispute{
		DisputeNo: fmt.Sprintf("DSP-%d", time.Now().UnixNano()/1e6),
		WaybillID: w.ID, Type: req.Type, Title: req.Title, Detail: req.Detail,
		Status: "open", CreatedByID: u.ID,
	}
	db.Create(&d)
	db.Preload("Waybill").Preload("CreatedBy").First(&d, d.ID)
	c.JSON(http.StatusOK, d)
}

func resolveDispute(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var req struct {
		Resolution string `json:"resolution"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Resolution == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请填写处理结果"})
		return
	}
	var d Dispute
	if err := db.First(&d, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "争议不存在"})
		return
	}
	db.Model(&d).Updates(map[string]interface{}{"status": "resolved", "resolution": req.Resolution})
	c.JSON(http.StatusOK, gin.H{"message": "争议已办结"})
}

// ---------- 本地模拟: 扫码 / 支付 ----------

// 模拟扫码枪: 输入运单号或回单编号(RC-<id>)返回对应信息
func mockScan(c *gin.Context) {
	var req struct {
		Code string `json:"code"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Code == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请提供扫码内容"})
		return
	}
	code := strings.TrimSpace(req.Code)
	if strings.HasPrefix(strings.ToUpper(code), "RC-") {
		rid, _ := strconv.Atoi(code[3:])
		var rc Receipt
		if err := db.Preload("Uploader").Preload("Attachments").First(&rc, rid).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "未找到回单: " + code})
			return
		}
		c.JSON(http.StatusOK, gin.H{"kind": "receipt", "receipt": rc})
		return
	}
	var w Waybill
	if err := db.Preload("Driver").Preload("Settlement").Where("waybill_no = ?", code).First(&w).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "未找到运单: " + code})
		return
	}
	c.JSON(http.StatusOK, gin.H{"kind": "waybill", "waybill": w})
}

// 模拟支付网关: 对原始结算或调整单付款,返回假交易号
func mockPay(c *gin.Context) {
	var req struct {
		Type string `json:"type"` // settlement / adjustment
		ID   uint   `json:"id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}
	now := time.Now()
	trade := fmt.Sprintf("PAY%s%04d", now.Format("20060102150405"), now.UnixNano()%10000)
	switch req.Type {
	case "settlement":
		var s Settlement
		if err := db.First(&s, req.ID).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "结算单不存在"})
			return
		}
		if s.Status == "paid" {
			c.JSON(http.StatusConflict, gin.H{"error": "该结算已支付,不能重复付款"})
			return
		}
		db.Model(&s).Updates(map[string]interface{}{"status": "paid", "paid_at": &now, "pay_trade_no": trade})
		c.JSON(http.StatusOK, gin.H{"message": "结算支付成功(模拟)", "trade_no": trade, "amount": s.Amount})
	case "adjustment":
		var a Adjustment
		if err := db.First(&a, req.ID).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "调整单不存在"})
			return
		}
		if a.Status == "paid" {
			c.JSON(http.StatusConflict, gin.H{"error": "该调整单已支付,不能重复付款"})
			return
		}
		db.Model(&a).Updates(map[string]interface{}{"status": "paid", "paid_at": &now, "pay_trade_no": trade})
		c.JSON(http.StatusOK, gin.H{"message": "调整差额支付成功(模拟)", "trade_no": trade, "amount": a.Difference})
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "type 必须为 settlement 或 adjustment"})
	}
}
