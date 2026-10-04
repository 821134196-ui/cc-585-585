package main

import (
	"crypto/rand"
	"encoding/hex"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

var db *gorm.DB

// 简单内存 token 存储: token -> userID
var (
	tokenMu sync.RWMutex
	tokens  = map[string]uint{}
)

func newToken(userID uint) string {
	b := make([]byte, 16)
	rand.Read(b)
	t := hex.EncodeToString(b)
	tokenMu.Lock()
	tokens[t] = userID
	tokenMu.Unlock()
	return t
}

func userByToken(t string) (uint, bool) {
	tokenMu.RLock()
	defer tokenMu.RUnlock()
	id, ok := tokens[t]
	return id, ok
}

// 认证中间件: 解析当前用户
func authRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		h := c.GetHeader("Authorization")
		if len(h) < 8 || h[:7] != "Bearer " {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "未登录"})
			return
		}
		uid, ok := userByToken(h[7:])
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "登录已失效"})
			return
		}
		var u User
		if err := db.First(&u, uid).Error; err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "用户不存在"})
			return
		}
		c.Set("user", &u)
		c.Next()
	}
}

func currentUser(c *gin.Context) *User {
	v, _ := c.Get("user")
	return v.(*User)
}

func requireRole(roles ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		u := currentUser(c)
		for _, r := range roles {
			if u.Role == r {
				c.Next()
				return
			}
		}
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "当前角色无权执行此操作(需要 " + joinRoles(roles) + ")"})
	}
}

func joinRoles(rs []string) string {
	s := ""
	for i, r := range rs {
		if i > 0 {
			s += "/"
		}
		s += r
	}
	return s
}

func main() {
	var err error
	dbPath := envOr("DB_PATH", "../data/logistics.db")
	if dir := filepath.Dir(dbPath); dir != "." {
		os.MkdirAll(dir, 0o755)
	}
	uploadDir := envOr("UPLOAD_DIR", "../uploads")
	os.MkdirAll(uploadDir, 0o755)

	db, err = gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		log.Fatal("数据库打开失败: ", err)
	}
	if err := db.AutoMigrate(&User{}, &Waybill{}, &Settlement{}, &Receipt{}, &Attachment{}, &RateVersion{}, &Adjustment{}, &Dispute{}); err != nil {
		log.Fatal("迁移失败: ", err)
	}
	seed(db)

	r := gin.Default()
	r.MaxMultipartMemory = 32 << 20

	api := r.Group("/api")
	{
		api.POST("/login", loginHandler)

		authed := api.Group("", authRequired())
		{
			authed.GET("/me", meHandler)
			authed.GET("/waybills", listWaybills)
			authed.GET("/waybills/:id", getWaybill)
			// 司机补录回单
			authed.POST("/waybills/:id/receipts", requireRole("driver"), uploadReceipt)
			// 运营审核
			authed.POST("/receipts/:id/review", requireRole("ops"), reviewReceipt)
			// 财务调整单
			authed.POST("/waybills/:id/adjustments", requireRole("finance"), createAdjustment)
			authed.GET("/adjustments", listAdjustments)
			authed.GET("/rate-versions", listRateVersions)
			authed.GET("/disputes", listDisputes)
			authed.POST("/waybills/:id/disputes", createDispute)
			authed.POST("/disputes/:id/resolve", requireRole("ops", "finance"), resolveDispute)
			// 本地模拟: 扫码与支付
			authed.POST("/mock/scan", mockScan)
			authed.POST("/mock/pay", mockPay)
		}
	}

	// 附件本地存储
	r.Static("/uploads", uploadDir)
	// 前端静态文件
	dist := envOr("WEB_DIST", "../web/dist")
	r.Static("/assets", filepath.Join(dist, "assets"))
	r.NoRoute(func(c *gin.Context) {
		p := filepath.Join(dist, "index.html")
		if _, err := os.Stat(p); err == nil {
			c.File(p)
		} else {
			c.String(http.StatusOK, "前端未构建。请先运行 ./start.sh 或 cd web && npm run build")
		}
	})

	addr := envOr("ADDR", ":8080")
	log.Println("服务启动: http://localhost" + addr)
	if err := r.Run(addr); err != nil {
		log.Fatal(err)
	}
}

func envOr(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
