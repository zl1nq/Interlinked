package models

import (
	"time"

	"gorm.io/gorm"
)

// Follow 描述用户之间的关注关系。
// 通过唯一索引 (user_id, followed_id) 保证同一关系仅一条记录。
type Follow struct {
	ID         uint           `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID     uint           `gorm:"index:idx_user_follow,unique;not null" json:"user_id"`     // 关注者
	FollowedID uint           `gorm:"index:idx_user_follow,unique;not null" json:"followed_id"` // 被关注者
	CreatedAt  time.Time      `json:"created_at"`
	DeletedAt  gorm.DeletedAt `gorm:"index" json:"-"`
}

func (Follow) TableName() string {
	return "follows"
}
