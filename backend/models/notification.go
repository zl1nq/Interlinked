package models

import "time"

const (
	NotificationTypeLike    = "like"
	NotificationTypeComment = "comment"
	NotificationTypeFollow  = "follow"
)

// Notification 通知中心实体，统一承载点赞/评论/关注等行为通知。
type Notification struct {
	ID        uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID    uint      `gorm:"index:idx_user_created;not null" json:"user_id"` // 接收通知的用户
	ActorID   uint      `gorm:"not null" json:"actor_id"`                       // 触发通知的用户
	Type      string    `gorm:"type:varchar(20);index:idx_user_created;not null" json:"type"`
	TargetID  uint      `gorm:"default:0" json:"target_id"` // 关联对象ID（如 feed_id）
	Content   string    `gorm:"type:varchar(500);default:''" json:"content"`
	IsRead    bool      `gorm:"index;default:false" json:"is_read"`
	CreatedAt time.Time `gorm:"index:idx_user_created" json:"created_at"`

	Actor User `gorm:"foreignKey:ActorID" json:"actor"`
}

func (Notification) TableName() string {
	return "notifications"
}
