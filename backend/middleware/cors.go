package middleware

import (
	"feed/config"
	"net/http"
	"net/url"
	"slices"
	"strings"

	"github.com/gin-gonic/gin"
)

const (
	allowOriginHeader      = "Access-Control-Allow-Origin"
	allowMethodsHeader     = "Access-Control-Allow-Methods"
	allowHeadersHeader     = "Access-Control-Allow-Headers"
	exposeHeadersHeader    = "Access-Control-Expose-Headers"
	allowCredentialsHeader = "Access-Control-Allow-Credentials"
)

// CorsMiddleware 跨域中间件
func CorsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		setCORSHeaders(c)
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

func setCORSHeaders(c *gin.Context) {
	origin := c.GetHeader("Origin")
	if IsAllowedOrigin(origin) {
		c.Header(allowOriginHeader, origin)
		c.Header(allowCredentialsHeader, "true")
	}
	c.Header(allowMethodsHeader, "GET, POST, PUT, DELETE, OPTIONS")
	c.Header(allowHeadersHeader, "Origin, Content-Type, Authorization, Accept")
	c.Header(exposeHeadersHeader, "Content-Length, Content-Type")
}

// IsAllowedOrigin 判断请求来源是否在配置白名单中。
func IsAllowedOrigin(origin string) bool {
	if origin == "" {
		return true
	}
	if slices.Contains(config.AppConfig.CORS.AllowOrigins, origin) {
		return true
	}
	if config.AppConfig.Server.Mode == "debug" {
		return isLocalDevOrigin(origin)
	}
	return false
}

func isLocalDevOrigin(origin string) bool {
	u, err := url.Parse(origin)
	if err != nil {
		return false
	}
	host := strings.ToLower(u.Hostname())
	return host == "localhost" || host == "127.0.0.1" || host == "::1"
}
