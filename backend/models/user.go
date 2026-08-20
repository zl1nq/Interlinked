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
	Avatar        string         `gorm:"type:varchar(500);default:''" json:"avatar"`
	Bio           string         `gorm:"type:varchar(500);default:''" json:"bio"`
	FollowerCount int64          `gorm:"default:0" json:"follower_count"` // 粉丝数
	FollowCount   int64          `gorm:"default:0" json:"follow_count"`   // 关注数
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
	Avatar        string `json:"avatar"`
	Bio           string `json:"bio"`
	FollowerCount int64  `json:"follower_count"`
	FollowCount   int64  `json:"follow_count"`
	IsBigV        bool   `json:"is_big_v"`
	IsFollowed    bool   `json:"is_followed"` // 当前登录用户是否关注
}

func (u *User) ToResponse() UserResponse {
	return UserResponse{
		ID:            u.ID,
		Username:      u.Username,
		Nickname:      u.Nickname,
		Avatar:        u.Avatar,
		Bio:           u.Bio,
		FollowerCount: u.FollowerCount,
		FollowCount:   u.FollowCount,
		IsBigV:        u.IsBigV,
	}
}
