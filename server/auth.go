package main

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// 简单 token 鉴权:Authorization: Bearer <token>
func authMiddleware(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		h := c.GetHeader("Authorization")
		token := strings.TrimPrefix(h, "Bearer ")
		if token == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "未登录"})
			return
		}
		var u User
		if err := db.Where("token = ?", token).First(&u).Error; err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "登录已失效"})
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

// 要求角色之一
func requireRole(roles ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		u := currentUser(c)
		for _, r := range roles {
			if u.Role == r {
				c.Next()
				return
			}
		}
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
			"error": "权限不足:该操作需要 " + strings.Join(roles, "/") + " 角色,当前为 " + roleName(u.Role),
		})
	}
}

func roleName(r string) string {
	switch r {
	case "driver":
		return "司机"
	case "ops":
		return "运营"
	case "finance":
		return "财务"
	case "admin":
		return "管理员"
	}
	return r
}
