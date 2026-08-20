package repository

import (
	"feed/models"
	"time"

	"gorm.io/gorm"
)

// FeedRepository 定义 Feed 域的数据访问契约。
// 约定：
// - service 层仅依赖该接口，不直接依赖 gorm 细节；
// - 事务由 BeginTx 开始，并在 service 层提交/回滚。
type FeedRepository interface {
	Create(feed *models.Feed) error
	CreateInTx(tx *gorm.DB, feed *models.Feed) error
	GetByID(feedID uint) (*models.Feed, error)
	GetByIDAndUserID(feedID, userID uint) (*models.Feed, error)
	UpdateByID(feedID uint, updates map[string]any) error
	Delete(feed *models.Feed) error

	IncreaseShareCount(tx *gorm.DB, feedID uint) error
	IncreaseLikeCount(tx *gorm.DB, feedID uint) error
	DecreaseLikeCount(tx *gorm.DB, feedID uint) error
	IncreaseCommentCount(tx *gorm.DB, feedID uint) error
	DecreaseCommentCount(tx *gorm.DB, feedID uint) error

	ListByUserID(userID uint, page, pageSize int) ([]models.Feed, int64, error)
	ListRecentByUserID(userID uint, limit int) ([]models.Feed, error)
	ListByIDs(feedIDs []uint) ([]models.Feed, error)
	ListByIDsBeforeCursor(feedIDs []uint, cursorTime time.Time, cursorID uint) ([]models.Feed, error)
	ListByIDsNewerThanCursor(feedIDs []uint, cursorTime time.Time, cursorID uint) ([]models.Feed, error)
	SearchByKeyword(keyword string, page, pageSize int) ([]models.Feed, int64, error)

	CreateTimeline(tx *gorm.DB, timeline *models.Timeline) error
	ListTimelineByUserID(userID uint, limit int) ([]models.Timeline, error)
	ListTimelinesByFeedID(feedID uint) ([]models.Timeline, error)
	DeleteTimelineByFeedID(feedID uint) error

	CreateLike(tx *gorm.DB, like *models.Like) error
	GetLike(userID, feedID uint) (*models.Like, error)
	DeleteLike(tx *gorm.DB, like *models.Like) error
	ListLikesByFeedID(feedID uint, page, pageSize int) ([]models.Like, int64, error)
	ListLikesByUserAndFeedIDs(userID uint, feedIDs []uint) ([]models.Like, error)

	CreateComment(tx *gorm.DB, comment *models.Comment) error
	GetCommentByIDAndFeedID(commentID, feedID uint) (*models.Comment, error)
	DeleteComment(tx *gorm.DB, comment *models.Comment) error
	ListCommentsByFeedID(feedID uint, page, pageSize int) ([]models.Comment, int64, error)

	BeginTx() *gorm.DB
}

type feedMySQLRepository struct {
	db *gorm.DB
}

func NewFeedRepository(db *gorm.DB) FeedRepository {
	return &feedMySQLRepository{db: db}
}

func (r *feedMySQLRepository) BeginTx() *gorm.DB { return r.db.Begin() }

func (r *feedMySQLRepository) Create(feed *models.Feed) error {
	return r.db.Create(feed).Error
}

func (r *feedMySQLRepository) CreateInTx(tx *gorm.DB, feed *models.Feed) error {
	return tx.Create(feed).Error
}

func (r *feedMySQLRepository) GetByID(feedID uint) (*models.Feed, error) {
	var feed models.Feed
	if err := r.db.Where("id = ?", feedID).First(&feed).Error; err != nil {
		return nil, err
	}
	return &feed, nil
}

func (r *feedMySQLRepository) GetByIDAndUserID(feedID, userID uint) (*models.Feed, error) {
	var feed models.Feed
	if err := r.db.Where("id = ? AND user_id = ?", feedID, userID).First(&feed).Error; err != nil {
		return nil, err
	}
	return &feed, nil
}

func (r *feedMySQLRepository) UpdateByID(feedID uint, updates map[string]any) error {
	return r.db.Model(&models.Feed{}).Where("id = ?", feedID).Updates(updates).Error
}

func (r *feedMySQLRepository) Delete(feed *models.Feed) error {
	return r.db.Delete(feed).Error
}

func (r *feedMySQLRepository) IncreaseShareCount(tx *gorm.DB, feedID uint) error {
	return tx.Model(&models.Feed{}).Where("id = ?", feedID).UpdateColumn("share_count", gorm.Expr("share_count + 1")).Error
}
func (r *feedMySQLRepository) IncreaseLikeCount(tx *gorm.DB, feedID uint) error {
	return tx.Model(&models.Feed{}).Where("id = ?", feedID).UpdateColumn("like_count", gorm.Expr("like_count + 1")).Error
}
func (r *feedMySQLRepository) DecreaseLikeCount(tx *gorm.DB, feedID uint) error {
	return tx.Model(&models.Feed{}).Where("id = ?", feedID).UpdateColumn("like_count", gorm.Expr("GREATEST(like_count - 1, 0)")).Error
}
func (r *feedMySQLRepository) IncreaseCommentCount(tx *gorm.DB, feedID uint) error {
	return tx.Model(&models.Feed{}).Where("id = ?", feedID).UpdateColumn("comment_count", gorm.Expr("comment_count + 1")).Error
}
func (r *feedMySQLRepository) DecreaseCommentCount(tx *gorm.DB, feedID uint) error {
	return tx.Model(&models.Feed{}).Where("id = ?", feedID).UpdateColumn("comment_count", gorm.Expr("GREATEST(comment_count - 1, 0)")).Error
}

func (r *feedMySQLRepository) ListByUserID(userID uint, page, pageSize int) ([]models.Feed, int64, error) {
	var feeds []models.Feed
	var total int64
	query := r.db.Model(&models.Feed{}).Where("user_id = ?", userID)
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	offset := (page - 1) * pageSize
	if err := query.Order("created_at DESC").Offset(offset).Limit(pageSize).Find(&feeds).Error; err != nil {
		return nil, 0, err
	}
	return feeds, total, nil
}

func (r *feedMySQLRepository) ListRecentByUserID(userID uint, limit int) ([]models.Feed, error) {
	var feeds []models.Feed
	if err := r.db.Where("user_id = ?", userID).Order("created_at DESC").Limit(limit).Find(&feeds).Error; err != nil {
		return nil, err
	}
	return feeds, nil
}

func (r *feedMySQLRepository) ListByIDs(feedIDs []uint) ([]models.Feed, error) {
	var feeds []models.Feed
	if len(feedIDs) == 0 {
		return feeds, nil
	}
	if err := r.db.Where("id IN ?", feedIDs).Find(&feeds).Error; err != nil {
		return nil, err
	}
	return feeds, nil
}

// 获取比游标时间旧的动态
func (r *feedMySQLRepository) ListByIDsBeforeCursor(feedIDs []uint, cursorTime time.Time, cursorID uint) ([]models.Feed, error) {
	var feeds []models.Feed
	if len(feedIDs) == 0 {
		return feeds, nil
	}
	if err := r.db.Where("id IN ? AND (created_at < ? OR (created_at = ? AND id < ?))", feedIDs, cursorTime, cursorTime, cursorID).
		Order("created_at DESC, id DESC").Find(&feeds).Error; err != nil {
		return nil, err
	}
	return feeds, nil
}

// 获取比游标时间新的动态
func (r *feedMySQLRepository) ListByIDsNewerThanCursor(feedIDs []uint, cursorTime time.Time, cursorID uint) ([]models.Feed, error) {
	var feeds []models.Feed
	if len(feedIDs) == 0 {
		return feeds, nil
	}
	if err := r.db.Where("id IN ? AND (created_at > ? OR (created_at = ? AND id > ?))", feedIDs, cursorTime, cursorTime, cursorID).
		Order("created_at DESC, id DESC").Find(&feeds).Error; err != nil {
		return nil, err
	}
	return feeds, nil
}

func (r *feedMySQLRepository) SearchByKeyword(keyword string, page, pageSize int) ([]models.Feed, int64, error) {
	var feeds []models.Feed
	var total int64
	// Phase1：先改为前缀匹配，避免 %keyword% 全表扫描。
	query := r.db.Model(&models.Feed{}).Where("content LIKE ?", keyword+"%")
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	offset := (page - 1) * pageSize
	if err := query.Order("created_at DESC").Offset(offset).Limit(pageSize).Find(&feeds).Error; err != nil {
		return nil, 0, err
	}
	return feeds, total, nil
}

func (r *feedMySQLRepository) CreateTimeline(tx *gorm.DB, timeline *models.Timeline) error {
	return tx.Create(timeline).Error
}

func (r *feedMySQLRepository) ListTimelineByUserID(userID uint, limit int) ([]models.Timeline, error) {
	var timelines []models.Timeline
	if err := r.db.Where("user_id = ?", userID).Order("created_at DESC").Limit(limit).Find(&timelines).Error; err != nil {
		return nil, err
	}
	return timelines, nil
}

func (r *feedMySQLRepository) ListTimelinesByFeedID(feedID uint) ([]models.Timeline, error) {
	var timelines []models.Timeline
	if err := r.db.Where("feed_id = ?", feedID).Find(&timelines).Error; err != nil {
		return nil, err
	}
	return timelines, nil
}

func (r *feedMySQLRepository) DeleteTimelineByFeedID(feedID uint) error {
	return r.db.Where("feed_id = ?", feedID).Delete(&models.Timeline{}).Error
}

func (r *feedMySQLRepository) CreateLike(tx *gorm.DB, like *models.Like) error {
	return tx.Create(like).Error
}

func (r *feedMySQLRepository) GetLike(userID, feedID uint) (*models.Like, error) {
	var like models.Like
	if err := r.db.Where("user_id = ? AND feed_id = ?", userID, feedID).First(&like).Error; err != nil {
		return nil, err
	}
	return &like, nil
}

func (r *feedMySQLRepository) DeleteLike(tx *gorm.DB, like *models.Like) error {
	return tx.Delete(like).Error
}

func (r *feedMySQLRepository) ListLikesByFeedID(feedID uint, page, pageSize int) ([]models.Like, int64, error) {
	var likes []models.Like
	var total int64
	query := r.db.Model(&models.Like{}).Where("feed_id = ?", feedID)
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	offset := (page - 1) * pageSize
	if err := query.Order("created_at DESC").Offset(offset).Limit(pageSize).Find(&likes).Error; err != nil {
		return nil, 0, err
	}
	return likes, total, nil
}

func (r *feedMySQLRepository) ListLikesByUserAndFeedIDs(userID uint, feedIDs []uint) ([]models.Like, error) {
	var likes []models.Like
	if len(feedIDs) == 0 {
		return likes, nil
	}
	if err := r.db.Where("user_id = ? AND feed_id IN ?", userID, feedIDs).Find(&likes).Error; err != nil {
		return nil, err
	}
	return likes, nil
}

func (r *feedMySQLRepository) CreateComment(tx *gorm.DB, comment *models.Comment) error {
	return tx.Create(comment).Error
}

func (r *feedMySQLRepository) GetCommentByIDAndFeedID(commentID, feedID uint) (*models.Comment, error) {
	var comment models.Comment
	if err := r.db.Where("id = ? AND feed_id = ?", commentID, feedID).First(&comment).Error; err != nil {
		return nil, err
	}
	return &comment, nil
}

func (r *feedMySQLRepository) DeleteComment(tx *gorm.DB, comment *models.Comment) error {
	return tx.Delete(comment).Error
}

func (r *feedMySQLRepository) ListCommentsByFeedID(feedID uint, page, pageSize int) ([]models.Comment, int64, error) {
	var comments []models.Comment
	var total int64
	query := r.db.Model(&models.Comment{}).Where("feed_id = ?", feedID)
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	offset := (page - 1) * pageSize
	if err := query.Order("created_at ASC").Offset(offset).Limit(pageSize).Find(&comments).Error; err != nil {
		return nil, 0, err
	}
	return comments, total, nil
}
