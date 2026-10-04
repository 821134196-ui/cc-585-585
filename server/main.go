package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func main() {
	root, _ := os.Getwd()
	// 允许从仓库根目录或 server/ 目录启动
	if _, err := os.Stat(filepath.Join(root, "server")); err == nil {
		// 在仓库根
	} else if _, err := os.Stat(filepath.Join(root, "go.mod")); err == nil {
		root = filepath.Dir(root) // 在 server/ 内,回退到仓库根
	}
	dataDir := filepath.Join(root, "data")
	uploadDir := filepath.Join(root, "uploads")
	webDist := filepath.Join(root, "web", "dist")
	ensureDir(dataDir)
	ensureDir(uploadDir)

	dbPath := filepath.Join(dataDir, "app.db")
	db, err := gorm.Open(sqlite.Open(dbPath+"?_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)"), &gorm.Config{})
	if err != nil {
		log.Fatal("打开数据库失败:", err)
	}
	if err := db.AutoMigrate(&User{}, &RateVersion{}, &Waybill{}, &Receipt{}, &Settlement{},
		&Payment{}, &Adjustment{}, &Dispute{}, &DisputeHandling{}, &AuditLog{}); err != nil {
		log.Fatal("迁移失败:", err)
	}
	// 部分唯一索引:同一运单最多一条"已审核"回单 —— 数据库层兜底"重复回单不能生成第二次签收"
	db.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_receipts_one_approved ON receipts(waybill_id) WHERE status = 'approved'`)
	// 结算金额不可变:禁止真正修改 amount(应用层本就不提供修改接口,触发器兜底)
	db.Exec(`DROP TRIGGER IF EXISTS trg_settlement_amount_immutable`)
	db.Exec(`CREATE TRIGGER trg_settlement_amount_immutable
		BEFORE UPDATE OF amount ON settlements
		WHEN OLD.amount <> NEW.amount
		BEGIN SELECT RAISE(ABORT, '原结算金额不可修改,请通过调整单处理差额'); END`)

	if err := seed(db); err != nil {
		log.Fatal("种子数据失败:", err)
	}

	h := &Handler{db: db, uploadDir: uploadDir}
	r := gin.Default()

	api := r.Group("/api")
	api.POST("/login", h.login)
	api.GET("/accounts", h.accounts) // 演示方便:公开账号列表

	auth := api.Group("")
	auth.Use(authMiddleware(db))
	auth.GET("/me", h.me)
	auth.GET("/waybills", h.listWaybills)
	auth.GET("/waybills/:id", h.waybillDetail)
	// 回单补录:司机(本人运单)或管理员
	auth.POST("/waybills/:id/receipts", requireRole("driver", "admin"), h.submitReceipt)
	// 回单审核:仅运营/管理员,且不能审自己提交的回单
	auth.POST("/receipts/:id/review", requireRole("ops", "admin"), h.reviewReceipt)
	auth.GET("/rate-versions", h.listRateVersions)
	auth.GET("/rate-versions/resolve", h.resolveRate)
	// 调整单:财务/管理员
	auth.POST("/waybills/:id/adjustments", requireRole("finance", "admin"), h.createAdjustment)
	// 模拟支付:财务/管理员
	auth.POST("/settlements/:id/pay", requireRole("finance", "admin"), h.paySettlement)
	// 模拟扫描:登录即可
	auth.POST("/scan/mock", h.scanMock)
	// 争议:登录即可发起;处理记录限运营/财务/管理员
	auth.POST("/waybills/:id/disputes", h.createDispute)
	auth.POST("/disputes/:id/handlings", requireRole("ops", "finance", "admin"), h.addDisputeHandling)
	auth.GET("/audit-logs", h.listAudits)

	// 附件与前端静态资源
	r.Static("/uploads", uploadDir)
	r.Static("/assets", filepath.Join(webDist, "assets"))
	r.NoRoute(func(c *gin.Context) {
		if strings.HasPrefix(c.Request.URL.Path, "/api/") {
			c.JSON(http.StatusNotFound, gin.H{"error": "接口不存在"})
			return
		}
		// SPA 兜底
		p := filepath.Join(webDist, filepath.Clean("/"+c.Request.URL.Path))
		if st, err := os.Stat(p); err == nil && !st.IsDir() && !strings.Contains(c.Request.URL.Path, "..") {
			c.File(p)
			return
		}
		c.File(filepath.Join(webDist, "index.html"))
	})

	addr := ":8080"
	if v := os.Getenv("ADDR"); v != "" {
		addr = v
	}
	fmt.Println("==============================================")
	fmt.Println(" 物流回单补录与运费争议系统")
	fmt.Println(" 访问地址: http://localhost" + addr)
	fmt.Println(" 演示账号: driver1/ops1/finance1,密码 123456")
	fmt.Println("==============================================")
	if err := r.Run(addr); err != nil {
		log.Fatal(err)
	}
}
