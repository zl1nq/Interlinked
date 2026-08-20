package models

import "time"

// Message 私信实体。
// 说明：
// - 会话由 (from_user_id, to_user_id) 双向聚合得到；
// - created_at 用于聊天记录与会话列表排序。
type Message struct {
	ID         uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	FromUserID uint      `gorm:"index:idx_conversation_time;not null" json:"from_user_id"`
	ToUserID   uint      `gorm:"index:idx_conversation_time;index:idx_message_to_read_created,priority:1;not null" json:"to_user_id"`
	Content    string    `gorm:"type:varchar(1000);not null" json:"content"`
	IsRead     bool      `gorm:"index:idx_message_to_read_created,priority:2;default:false" json:"is_read"`
	CreatedAt  time.Time `gorm:"index:idx_conversation_time;index:idx_message_to_read_created,priority:3" json:"created_at"`

	FromUser User `gorm:"foreignKey:FromUserID" json:"from_user"`
	ToUser   User `gorm:"foreignKey:ToUserID" json:"to_user"`
}

func (Message) TableName() string {
	return "messages"
}

// Conversation 为每个用户维护一条会话聚合记录，用于高效分页会话列表。
type Conversation struct {
	ID                 uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID             uint      `gorm:"index:idx_conversation_user_target,unique,priority:1;index:idx_conversation_user_last,priority:1;not null" json:"user_id"`
	TargetUserID       uint      `gorm:"index:idx_conversation_user_target,unique,priority:2;not null" json:"target_user_id"`
	LastMessageID      uint      `gorm:"not null" json:"last_message_id"`
	LastMessageContent string    `gorm:"type:varchar(1000);not null" json:"last_message_content"`
	LastMessageAt      time.Time `gorm:"index:idx_conversation_user_last,priority:2;not null" json:"last_message_at"`
	UnreadCount        int64     `gorm:"default:0;not null" json:"unread_count"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`

	TargetUser User `gorm:"foreignKey:TargetUserID" json:"target_user"`
}

func (Conversation) TableName() string {
	return "conversations"
}
