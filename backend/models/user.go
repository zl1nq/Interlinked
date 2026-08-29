// Package models 定义数据库实体与对外响应结构。
package models

import (
	"time"

	"gorm.io/gorm"
)

// User 用户模型
type User struct {
	ID            uint           `gorm:"primaryKey;autoIncrement" json:"id"`
	Username      string         `gorm:"type:varchar(50);uniqueIndex;index:idx_user_search_username;not null" json:"username"`
	Password      string         `gorm:"type:varchar(255);not null" json:"-"`
	Nickname      string         `gorm:"type:varchar(100);index:idx_user_search_nickname;not null" json:"nickname"`
	Email         *string        `gorm:"type:varchar(255);uniqueIndex" json:"email"` // 唯一邮箱；老数据可为 NULL
	EmailVerified bool           `gorm:"default:false" json:"email_verified"`
	TokenVersion  int            `gorm:"default:1;not null" json:"-"` // 令牌版本：改密后 +1 吊销所有旧 JWT
	Avatar        string         `gorm:"type:varchar(500);default:''" json:"avatar"`
	Bio           string         `gorm:"type:varchar(500);default:''" json:"bio"`
	FollowerCount int64          `gorm:"default:0" json:"follower_count"` // 粉丝数
	FollowCount   int64          `gorm:"default:0" json:"follow_count"`   // 关注数a
	IsBigV        bool           `gorm:"default:false" json:"is_big_v"`   // 是否为大V（粉丝数超过阈值）
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
	DeletedAt     gorm.DeletedAt `gorm:"index" json:"-"`
}

func (User) TableName() string {
	return "users"
}

// UserResponse 用户响应（不包含敏感信息）
type UserResponse struct {
	ID            uint   `json:"id"`
	Username      string `json:"username"`
	Nickname      string `json:"nickname"`
	Email         string `json:"email"`
	EmailVerified bool   `json:"email_verified"`
	Avatar        string `json:"avatar"`
	Bio           string `json:"bio"`
	FollowerCount int64  `json:"follower_count"`
	FollowCount   int64  `json:"follow_count"`
	IsBigV        bool   `json:"is_big_v"`
	IsFollowed    bool   `json:"is_followed"` // 当前登录用户是否关注
}

func (u *User) ToResponse() UserResponse {
	var email string
	if u.Email != nil {
		email = *u.Email
	}
	return UserResponse{
		ID:            u.ID,
		Username:      u.Username,
		Nickname:      u.Nickname,
		Email:         email,
		EmailVerified: u.EmailVerified,
		Avatar:        u.Avatar,
		Bio:           u.Bio,
		FollowerCount: u.FollowerCount,
		FollowCount:   u.FollowCount,
		IsBigV:        u.IsBigV,
	}
}
