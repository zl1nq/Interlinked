package repository

import (
	"feed/models"

	"gorm.io/gorm"
)

// NotificationRepository 定义通知中心数据访问契约。
type NotificationRepository interface {
	Create(notification *models.Notification) error
	ListByUser(userID uint, page, pageSize int) ([]models.Notification, int64, error)
	CountUnread(userID uint) (int64, error)
	MarkAllRead(userID uint) error
}

type notificationMySQLRepository struct {
	db *gorm.DB
}

func NewNotificationRepository(db *gorm.DB) NotificationRepository {
	return &notificationMySQLRepository{db: db}
}

func (r *notificationMySQLRepository) Create(notification *models.Notification) error {
	return r.db.Create(notification).Error
}

func (r *notificationMySQLRepository) ListByUser(userID uint, page, pageSize int) ([]models.Notification, int64, error) {
	var list []models.Notification
	var total int64

	query := r.db.Model(&models.Notification{}).Where("user_id = ?", userID)
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize
	if err := query.Preload("Actor").Order("created_at DESC").Offset(offset).Limit(pageSize).Find(&list).Error; err != nil {
		return nil, 0, err
	}

	return list, total, nil
}

func (r *notificationMySQLRepository) CountUnread(userID uint) (int64, error) {
	var count int64
	err := r.db.Model(&models.Notification{}).Where("user_id = ? AND is_read = ?", userID, false).Count(&count).Error
	return count, err
}

func (r *notificationMySQLRepository) MarkAllRead(userID uint) error {
	return r.db.Model(&models.Notification{}).Where("user_id = ? AND is_read = ?", userID, false).Update("is_read", true).Error
}
