package models

import "time"

// Visit 主页访问记录
// visitor_id 访问者，target_user_id 被访问者
type Visit struct {
	ID           uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	VisitorID    uint      `gorm:"not null;index:idx_target_visited;uniqueIndex:uk_visitor_target" json:"visitor_id"`
	TargetUserID uint      `gorm:"not null;index:idx_target_visited;uniqueIndex:uk_visitor_target" json:"target_user_id"`
	VisitedAt    time.Time `gorm:"not null;index:idx_target_visited" json:"visited_at"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`

	Visitor User `gorm:"foreignKey:VisitorID" json:"visitor"`
}

func (Visit) TableName() string {
	return "visits"
}
