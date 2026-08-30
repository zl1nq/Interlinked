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
	ListPopularFeeds(page, pageSize int) ([]models.Feed, int64, error)
	ListTopFeedIDs(limit int) ([]uint, int64, error)
	SearchByKeyword(keyword string, page, pageSize int) ([]models.Feed, int64, error)

	CreateTimeline(tx *gorm.DB, timeline *models.Timeline) error
	ListTimelineByUserID(userID uint, limit int) ([]models.Timeline, error)
	ListTimelinesByFeedID(feedID uint) ([]models.Timeline, error)
	DeleteTimelineByFeedID(feedID uint) error

	CreateLike(tx *gorm.DB, like *models.Like) error
	GetLike(userID, feedID uint) (*models.Like, error)
	DeleteLike(tx *gorm.DB, like *models.Like) error
	DeleteLikesByFeedID(feedID uint) error
	ListLikesByFeedID(feedID uint, page, pageSize int) ([]models.Like, int64, error)
	ListLikesByUserAndFeedIDs(userID uint, feedIDs []uint) ([]models.Like, error)

	CreateComment(tx *gorm.DB, comment *models.Comment) error
	GetCommentByIDAndFeedID(commentID, feedID uint) (*models.Comment, error)
	DeleteComment(tx *gorm.DB, comment *models.Comment) error
	DeleteCommentsByFeedID(feedID uint) error
	ListCommentsByFeedID(feedID uint, page, pageSize int) ([]models.Comment, int64, error)

	ListRootCommentsByFeedID(feedID uint, page, pageSize int) ([]models.Comment, int64, error)
	CountRepliesByRootIDs(rootIDs []uint) (map[uint]int64, error)
	ListRepliesByRootIDs(rootIDs []uint) ([]models.Comment, error)
	ListRepliesByRootID(rootID uint, page, pageSize int) ([]models.Comment, int64, error)
	DeleteRepliesByRootID(tx *gorm.DB, rootID uint) (int64, error)
	DecreaseCommentCountBy(tx *gorm.DB, feedID uint, n int64) error

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

// CreateInTx 插入一条 Feed 记录到事务中；事务版本（使用外部传入的 tx 连接）
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

// ListTopFeedIDs 点赞数 Top N 动态的 ID（全站排行，原创+转发同榜），供发现页缓存回源。
// deleted_at IS NULL 为显式声明（GORM 软删过滤本会自动追加，写出便于阅读）。
func (r *feedMySQLRepository) ListTopFeedIDs(limit int) ([]uint, int64, error) {
	var total int64
	if err := r.db.Model(&models.Feed{}).Where("deleted_at IS NULL").Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var ids []uint
	if err := r.db.Model(&models.Feed{}).Select("id").Where("deleted_at IS NULL").
		Order("like_count DESC, id DESC").Limit(limit).Pluck("id", &ids).Error; err != nil {
		return nil, 0, err
	}
	return ids, total, nil
}

// ListPopularFeeds 点赞数全站排行（原创+转发同榜），按 like_count DESC, id DESC 稳定排序。
// deleted_at IS NULL 为显式声明（GORM 软删过滤本会自动追加，写出便于阅读）。
func (r *feedMySQLRepository) ListPopularFeeds(page, pageSize int) ([]models.Feed, int64, error) {
	var feeds []models.Feed
	var total int64
	query := r.db.Model(&models.Feed{}) //.Where("deleted_at IS NULL")
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	offset := (page - 1) * pageSize
	if err := query.Order("like_count DESC, id DESC").Offset(offset).Limit(pageSize).Find(&feeds).Error; err != nil {
		return nil, 0, err
	}
	return feeds, total, nil
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

// DeleteLikesByFeedID 物理删除动态的全部点赞记录（Like 无软删字段，Delete 即物理删）。
func (r *feedMySQLRepository) DeleteLikesByFeedID(feedID uint) error {
	return r.db.Where("feed_id = ?", feedID).Delete(&models.Like{}).Error
}

// DeleteCommentsByFeedID 物理删除动态的全部评论（Comment 有软删字段，用 Unscoped 绕过）。
func (r *feedMySQLRepository) DeleteCommentsByFeedID(feedID uint) error {
	return r.db.Unscoped().Where("feed_id = ?", feedID).Delete(&models.Comment{}).Error
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

// ListRootCommentsByFeedID 分页获取动态的根评论（root_id=0），时间正序。
func (r *feedMySQLRepository) ListRootCommentsByFeedID(feedID uint, page, pageSize int) ([]models.Comment, int64, error) {
	var comments []models.Comment
	var total int64
	query := r.db.Model(&models.Comment{}).Where("feed_id = ? AND root_id = 0", feedID)
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	offset := (page - 1) * pageSize
	if err := query.Order("created_at ASC, id ASC").Offset(offset).Limit(pageSize).Find(&comments).Error; err != nil {
		return nil, 0, err
	}
	return comments, total, nil
}

// CountRepliesByRootIDs 统计每层楼的回复数（不含根评论自身），返回 root_id -> 回复数。
func (r *feedMySQLRepository) CountRepliesByRootIDs(rootIDs []uint) (map[uint]int64, error) {
	counts := make(map[uint]int64, len(rootIDs))
	if len(rootIDs) == 0 {
		return counts, nil
	}
	type row struct {
		RootID uint  `json:"root_id"`
		Cnt    int64 `json:"cnt"`
	}
	var rows []row
	if err := r.db.Model(&models.Comment{}).
		Select("root_id, COUNT(*) AS cnt").
		Where("root_id IN ?", rootIDs).
		Group("root_id").Find(&rows).Error; err != nil {
		return nil, err
	}
	for _, rw := range rows {
		counts[rw.RootID] = rw.Cnt
	}
	return counts, nil
}

// ListRepliesByRootIDs 批量取各楼层全部回复（时间正序），供首屏"前 3 条"在内存中切分。
func (r *feedMySQLRepository) ListRepliesByRootIDs(rootIDs []uint) ([]models.Comment, error) {
	if len(rootIDs) == 0 {
		return []models.Comment{}, nil
	}
	var replies []models.Comment
	if err := r.db.Where("root_id IN ?", rootIDs).
		Order("root_id ASC, created_at ASC, id ASC").Find(&replies).Error; err != nil {
		return nil, err
	}
	return replies, nil
}

// ListRepliesByRootID 楼内回复按时间正序分页（根评论 ID 视角）。
func (r *feedMySQLRepository) ListRepliesByRootID(rootID uint, page, pageSize int) ([]models.Comment, int64, error) {
	var replies []models.Comment
	var total int64
	query := r.db.Model(&models.Comment{}).Where("root_id = ?", rootID)
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	offset := (page - 1) * pageSize
	if err := query.Order("created_at ASC, id ASC").Offset(offset).Limit(pageSize).Find(&replies).Error; err != nil {
		return nil, 0, err
	}
	return replies, total, nil
}

// DeleteRepliesByRootID 软删除整楼的全部回复（根评论删除时的级联），返回删除行数，幂等。
func (r *feedMySQLRepository) DeleteRepliesByRootID(tx *gorm.DB, rootID uint) (int64, error) {
	res := tx.Where("root_id = ?", rootID).Delete(&models.Comment{})
	return res.RowsAffected, res.Error
}

// DecreaseCommentCountBy 按数量递减动态评论数（整楼级联删除时一次减去 1+N）。
func (r *feedMySQLRepository) DecreaseCommentCountBy(tx *gorm.DB, feedID uint, n int64) error {
	return tx.Model(&models.Feed{}).Where("id = ?", feedID).
		UpdateColumn("comment_count", gorm.Expr("GREATEST(comment_count - ?, 0)", n)).Error
}
