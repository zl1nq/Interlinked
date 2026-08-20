package models

import "time"

const (
	OutboxEventStatusPending = "pending"
	OutboxEventStatusSent    = "sent"
	OutboxEventStatusFailed  = "failed"
	OutboxEventStatusDead    = "dead"
)

const (
	OutboxEventTypeFeedPublished = "feed.published"
	OutboxEventTypeFeedDeleted   = "feed.deleted"
)

// OutboxEvent 可靠事件表，用于解决业务写库与异步投递之间的一致性问题。
type OutboxEvent struct {
	ID          uint       `gorm:"primaryKey;autoIncrement" json:"id"`
	EventType   string     `gorm:"type:varchar(64);index:idx_outbox_status_next,priority:2;not null" json:"event_type"`
	AggregateID uint       `gorm:"index;not null" json:"aggregate_id"`
	Payload     string     `gorm:"type:text;not null" json:"payload"`
	Status      string     `gorm:"type:varchar(20);index:idx_outbox_status_next,priority:1;default:'pending';not null" json:"status"`
	RetryCount  int        `gorm:"default:0;not null" json:"retry_count"`
	LastError   string     `gorm:"type:varchar(1000);default:''" json:"last_error"`
	NextRetryAt time.Time  `gorm:"index:idx_outbox_status_next,priority:3" json:"next_retry_at"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	SentAt      *time.Time `json:"sent_at"`
}

func (OutboxEvent) TableName() string {
	return "outbox_events"
}
