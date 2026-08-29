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
	DeleteByUserAndID(userID, notificationID uint) (int64, error)
	DeleteReadByUser(userID uint) (int64, error)
	DeleteByTargetFeed(feedID uint) (int64, error)
	DeleteByActorTarget(actorID, receiverID, targetID uint, notificationType string) (int64, error)
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

// DeleteByUserAndID 物理删除指定通知（无论是否已读），仅限接收者本人；返回删除行数。
func (r *notificationMySQLRepository) DeleteByUserAndID(userID, notificationID uint) (int64, error) {
	res := r.db.Where("id = ? AND user_id = ?", notificationID, userID).Delete(&models.Notification{})
	return res.RowsAffected, res.Error
}

// DeleteReadByUser 物理删除该用户全部已读通知（未读保留）；返回删除行数。
func (r *notificationMySQLRepository) DeleteReadByUser(userID uint) (int64, error) {
	res := r.db.Where("user_id = ? AND is_read = ?", userID, true).Delete(&models.Notification{})
	return res.RowsAffected, res.Error
}

// DeleteByTargetFeed 物理删除指向某动态的全部点赞/评论通知（动态删除时的级联清理）。
func (r *notificationMySQLRepository) DeleteByTargetFeed(feedID uint) (int64, error) {
	res := r.db.Where("target_id = ? AND type IN ?", feedID,
		[]string{models.NotificationTypeLike, models.NotificationTypeComment}).Delete(&models.Notification{})
	return res.RowsAffected, res.Error
}

// DeleteByActorTarget 精确撤回一条行为通知：
// - 取消点赞：actor=取消者, receiver=动态作者, target=feedID, type=like
// - 删除评论：同上, type=comment
// - 取消关注：actor=取消关注者, receiver=被取关者, target=0, type=follow
func (r *notificationMySQLRepository) DeleteByActorTarget(actorID, receiverID, targetID uint, notificationType string) (int64, error) {
	res := r.db.Where("user_id = ? AND actor_id = ? AND target_id = ? AND type = ?",
		receiverID, actorID, targetID, notificationType).Delete(&models.Notification{})
	return res.RowsAffected, res.Error
}
