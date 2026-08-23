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

// CreateInTx 插入一条 OutboxEvent 记录到事务中
func (r *outboxMySQLRepository) CreateInTx(tx *gorm.DB, event *models.OutboxEvent) error {
	return tx.Create(event).Error
}

func (r *outboxMySQLRepository) ListDue(limit int) ([]models.OutboxEvent, error) {
	if limit <= 0 {
		limit = 50 // 默认每次拉取 50 条
	}
	var events []models.OutboxEvent
	err := r.db.Transaction(func(tx *gorm.DB) error {
		return tx.
			Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}). // ① 行级排他锁 + 跳过已锁定行
			Where("status IN ? AND next_retry_at <= ?",
								[]string{models.OutboxEventStatusPending, models.OutboxEventStatusFailed}, time.Now()). // ② 筛选待投递事件
			Order("next_retry_at ASC, id ASC"). // ③ 按优先级排序
			Limit(limit).                       // ④ 限制批量
			Find(&events).Error
	})
	return events, err
}

// MarkSent 将事件标记为已成功投递。
func (r *outboxMySQLRepository) MarkSent(eventID uint) error {
	now := time.Now()
	//指定操作的目标模型为 OutboxEvent 对应的数据库表
	return r.db.Model(&models.OutboxEvent{}).
		//按主键定位要更新的事件记录
		Where("id = ?", eventID).
		//执行批量字段更新
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
