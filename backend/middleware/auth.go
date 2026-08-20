package middleware

import (
	"errors"
	"feed/utils"
	"strings"

	"github.com/gin-gonic/gin"
)

const (
	authorizationHeader = "Authorization"
	bearerPrefix        = "Bearer"
)

var errTokenMissing = errors.New("token missing")

// AuthMiddleware JWT认证中间件
func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		claims, err := ParseTokenFromRequest(c)
		if err != nil {
			utils.Unauthorized(c, "Token无效或已过期")
			c.Abort()
			return
		}

		c.Set("user_id", claims.UserID)
		c.Set("username", claims.Username)
		c.Next()
	}
}

// extractBearerToken 提取 Bearer Token
func extractBearerToken(authHeader string) (string, bool) {
	authHeader = strings.TrimSpace(authHeader)
	if authHeader == "" {
		return "", false
	}

	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 || parts[0] != bearerPrefix || strings.TrimSpace(parts[1]) == "" {
		return "", false
	}

	return parts[1], true
}

// GetCurrentUserID 从上下文获取当前用户ID
func GetCurrentUserID(c *gin.Context) uint {
	userID, exists := c.Get("user_id")
	if !exists {
		return 0
	}
	if id, ok := userID.(uint); ok {
		return id
	}
	return 0
}

// ParseTokenFromQuery 解析 query 中携带的 JWT（WebSocket 场景）。
func ParseTokenFromQuery(token string) (*utils.Claims, error) {
	return utils.ParseToken(token)
}

// ParseTokenFromRequest 统一从请求中提取 token：
// 1) Authorization: Bearer xxx
// 2) query: ?token=xxx（WS 场景兜底）
func ParseTokenFromRequest(c *gin.Context) (*utils.Claims, error) {
	if tokenString, ok := extractBearerToken(c.GetHeader(authorizationHeader)); ok {
		return utils.ParseToken(tokenString)
	}
	if token := strings.TrimSpace(c.Query("token")); token != "" {
		return utils.ParseToken(token)
	}
	return nil, errTokenMissing
}
