package middleware

import (
	"github.com/gin-gonic/gin"
	"smart-bill-manager/internal/utils"
)

// RequireAdmin allows only admin users to access an endpoint.
func RequireAdmin() gin.HandlerFunc {
	return func(c *gin.Context) {
		role := GetUserRole(c)
		if role != "admin" {
			utils.Error(c, 403, "需要管理员权限", nil)
			c.Abort()
			return
		}
		c.Next()
	}
}

