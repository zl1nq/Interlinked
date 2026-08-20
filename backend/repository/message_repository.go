package repository

import (
	"feed/models"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// MessageRepository 定义私信域数据访问契约。
type MessageRepository interface {
	Create(message *models.Message) error
	CreateWithConversations(message *models.Message) error
	UpsertConversations(message *models.Message) error
	EnsureConversationFromHistory(currentUserID, targetUserID uint) error
	GetByConversation(currentUserID, targetUserID uint, page, pageSize int) ([]models.Message, int64, error)
	ListConversations(userID uint, page, pageSize int) ([]models.Conversation, int64, error)
	CountUnreadFromTo(fromUserID, toUserID uint) (int64, error)
	MarkReadFromTo(fromUserID, toUserID uint) error
}

type messageMySQLRepository struct {
	db *gorm.DB
}

func NewMessageRepository(db *gorm.DB) MessageRepository {
	return &messageMySQLRepository{db: db}
}

func (r *messageMySQLRepository) Create(message *models.Message) error {
	return r.db.Create(message).Error
}

func (r *messageMySQLRepository) CreateWithConversations(message *models.Message) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(message).Error; err != nil {
			return err
		}
		return upsertConversationsInTx(tx, message)
	})
}

func (r *messageMySQLRepository) UpsertConversations(message *models.Message) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		return upsertConversationsInTx(tx, message)
	})
}

func upsertConversationsInTx(tx *gorm.DB, message *models.Message) error {
	if err := upsertConversation(tx, message.FromUserID, message.ToUserID, message, 0); err != nil {
		return err
	}
	return upsertConversation(tx, message.ToUserID, message.FromUserID, message, 1)
}

func upsertConversation(tx *gorm.DB, userID, targetUserID uint, message *models.Message, unreadDelta int64) error {
	conversation := models.Conversation{
		UserID:             userID,
		TargetUserID:       targetUserID,
		LastMessageID:      message.ID,
		LastMessageContent: message.Content,
		LastMessageAt:      message.CreatedAt,
		UnreadCount:        unreadDelta,
	}
	return tx.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "user_id"}, {Name: "target_user_id"}},
		DoUpdates: clause.Assignments(map[string]any{
			"last_message_id":      message.ID,
			"last_message_content": message.Content,
			"last_message_at":      message.CreatedAt,
			"unread_count":         gorm.Expr("unread_count + ?", unreadDelta),
		}),
	}).Create(&conversation).Error
}

func (r *messageMySQLRepository) EnsureConversationFromHistory(currentUserID, targetUserID uint) error {
	var latest models.Message
	if err := r.db.Where("(from_user_id = ? AND to_user_id = ?) OR (from_user_id = ? AND to_user_id = ?)",
		currentUserID, targetUserID, targetUserID, currentUserID).
		Order("created_at DESC, id DESC").
		First(&latest).Error; err != nil {
		return err
	}
	return r.UpsertConversations(&latest)
}

func (r *messageMySQLRepository) GetByConversation(currentUserID, targetUserID uint, page, pageSize int) ([]models.Message, int64, error) {
	var messages []models.Message
	var total int64

	query := r.db.Model(&models.Message{}).
		Where("(from_user_id = ? AND to_user_id = ?) OR (from_user_id = ? AND to_user_id = ?)",
			currentUserID, targetUserID, targetUserID, currentUserID)

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize
	if err := query.Preload("FromUser").Preload("ToUser").Order("created_at DESC").Offset(offset).Limit(pageSize).Find(&messages).Error; err != nil {
		return nil, 0, err
	}

	return messages, total, nil
}

func (r *messageMySQLRepository) ListConversations(userID uint, page, pageSize int) ([]models.Conversation, int64, error) {
	var conversations []models.Conversation
	var total int64
	query := r.db.Model(&models.Conversation{}).Where("user_id = ?", userID)
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	offset := (page - 1) * pageSize
	if err := query.Preload("TargetUser").Order("last_message_at DESC, id DESC").Offset(offset).Limit(pageSize).Find(&conversations).Error; err != nil {
		return nil, 0, err
	}
	return conversations, total, nil
}

func (r *messageMySQLRepository) CountUnreadFromTo(fromUserID, toUserID uint) (int64, error) {
	var count int64
	err := r.db.Model(&models.Message{}).
		Where("from_user_id = ? AND to_user_id = ? AND is_read = ?", fromUserID, toUserID, false).
		Count(&count).Error
	return count, err
}

func (r *messageMySQLRepository) MarkReadFromTo(fromUserID, toUserID uint) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&models.Message{}).
			Where("from_user_id = ? AND to_user_id = ? AND is_read = ?", fromUserID, toUserID, false).
			Update("is_read", true).Error; err != nil {
			return err
		}
		return tx.Model(&models.Conversation{}).
			Where("user_id = ? AND target_user_id = ?", toUserID, fromUserID).
			Update("unread_count", 0).Error
	})
}
