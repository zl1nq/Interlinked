package utils

import (
	"feed/config"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Claims 定义 JWT 载荷字段。
// 仅放鉴权必需信息，避免放敏感业务数据。
type Claims struct {
	UserID   uint   `json:"user_id"`
	Username string `json:"username"`
	Scope    string `json:"scope,omitempty"`
	jwt.RegisteredClaims
}

// GenerateToken 生成登录 JWT Token。
func GenerateToken(userID uint, username string) (string, error) {
	expireHours := config.AppConfig.JWT.Expire
	claims := Claims{
		UserID:   userID,
		Username: username,
		Scope:    "api",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Duration(expireHours) * time.Hour)), //过期时间
			IssuedAt:  jwt.NewNumericDate(time.Now()),                                             //签发时间
			Issuer:    "feed",                                                                     //签发人
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(config.AppConfig.JWT.Secret))
}

// ParseToken 解析JWT Token
func ParseToken(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (any, error) {
		return []byte(config.AppConfig.JWT.Secret), nil
	})

	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(*Claims); ok && token.Valid {
		return claims, nil
	}

	return nil, jwt.ErrSignatureInvalid
}
