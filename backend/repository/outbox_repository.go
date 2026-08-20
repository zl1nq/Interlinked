package repository

import (
	"feed/models"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// OutboxRepository 定义可靠事件表的数据访问契约。
type OutboxRepository interface {
	CreateInTx(tx *gorm.DB, event *models.OutboxEvent) error
	ListDue(limit int) ([]models.OutboxEvent, error)
	MarkSent(eventID uint) error
	MarkFailed(eventID uint, retryCount int, lastErr string, nextRetryAt time.Time) error
	MarkDead(eventID uint, retryCount int, lastErr string) error
}

type outboxMySQLRepository struct {
	db *gorm.DB
}

func NewOutboxRepository(db *gorm.DB) OutboxRepository {
	return &outboxMySQLRepository{db: db}
}

func (r *outboxMySQLRepository) CreateInTx(tx *gorm.DB, event *models.OutboxEvent) error {
	return tx.Create(event).Error
}

func (r *outboxMySQLRepository) ListDue(limit int) ([]models.OutboxEvent, error) {
	if limit <= 0 {
		limit = 50
	}
	var events []models.OutboxEvent
	err := r.db.Transaction(func(tx *gorm.DB) error {
		return tx.
			Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}).
			Where("status IN ? AND next_retry_at <= ?", []string{models.OutboxEventStatusPending, models.OutboxEventStatusFailed}, time.Now()).
			Order("next_retry_at ASC, id ASC").
			Limit(limit).
			Find(&events).Error
	})
	return events, err
}

func (r *outboxMySQLRepository) MarkSent(eventID uint) error {
	now := time.Now()
	return r.db.Model(&models.OutboxEvent{}).
		Where("id = ?", eventID).
		Updates(map[string]any{
			"status":  models.OutboxEventStatusSent,
			"sent_at": &now,
		}).Error
}

func (r *outboxMySQLRepository) MarkFailed(eventID uint, retryCount int, lastErr string, nextRetryAt time.Time) error {
	return r.db.Model(&models.OutboxEvent{}).
		Where("id = ?", eventID).
		Updates(map[string]any{
			"status":        models.OutboxEventStatusFailed,
			"retry_count":   retryCount,
			"last_error":    lastErr,
			"next_retry_at": nextRetryAt,
		}).Error
}

func (r *outboxMySQLRepository) MarkDead(eventID uint, retryCount int, lastErr string) error {
	return r.db.Model(&models.OutboxEvent{}).
		Where("id = ?", eventID).
		Updates(map[string]any{
			"status":      models.OutboxEventStatusDead,
			"retry_count": retryCount,
			"last_error":  lastErr,
		}).Error
}
