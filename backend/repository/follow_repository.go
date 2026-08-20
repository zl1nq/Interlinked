package repository

import (
	"feed/models"

	"gorm.io/gorm"
)

type FollowRepository interface {
	GetTargetUser(followedID uint) (*models.User, error)
	GetAnyRelation(userID, followedID uint) (*models.Follow, error)
	CreateRelation(tx *gorm.DB, follow *models.Follow) error
	RestoreRelation(tx *gorm.DB, relationID uint) error
	DeleteRelation(tx *gorm.DB, follow *models.Follow) error
	IncreaseFollowCount(tx *gorm.DB, userID uint) error
	IncreaseFollowerCount(tx *gorm.DB, userID uint) error
	DecreaseFollowCount(tx *gorm.DB, userID uint) error
	DecreaseFollowerCount(tx *gorm.DB, userID uint) error
	ListFollowers(userID uint, page, pageSize int) ([]models.Follow, int64, error)
	ListFollowing(userID uint, page, pageSize int) ([]models.Follow, int64, error)
	ListFollowingAll(userID uint) ([]models.Follow, error)
	ListUsersByIDs(userIDs []uint) ([]models.User, error)
	ListRecentFeedsByUser(userID uint, limit int) ([]models.Feed, error)
	ListFeedIDsByUser(userID uint) ([]models.Feed, error)
	CreateTimelines(tx *gorm.DB, timelines []models.Timeline) error
	DeleteTimelineByUserAndAuthor(userID, authorID uint) error
	BeginTx() *gorm.DB
}

type followMySQLRepository struct {
	db *gorm.DB
}

func NewFollowRepository(db *gorm.DB) FollowRepository {
	return &followMySQLRepository{db: db}
}

func (r *followMySQLRepository) BeginTx() *gorm.DB { return r.db.Begin() }

func (r *followMySQLRepository) GetTargetUser(followedID uint) (*models.User, error) {
	var user models.User
	if err := r.db.Where("id = ?", followedID).First(&user).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *followMySQLRepository) GetAnyRelation(userID, followedID uint) (*models.Follow, error) {
	var follow models.Follow
	if err := r.db.Unscoped().Where("user_id = ? AND followed_id = ?", userID, followedID).First(&follow).Error; err != nil {
		return nil, err
	}
	return &follow, nil
}

func (r *followMySQLRepository) CreateRelation(tx *gorm.DB, follow *models.Follow) error {
	return tx.Create(follow).Error
}

func (r *followMySQLRepository) RestoreRelation(tx *gorm.DB, relationID uint) error {
	return tx.Unscoped().Model(&models.Follow{}).Where("id = ?", relationID).Update("deleted_at", nil).Error
}

func (r *followMySQLRepository) DeleteRelation(tx *gorm.DB, follow *models.Follow) error {
	return tx.Delete(follow).Error
}

func (r *followMySQLRepository) IncreaseFollowCount(tx *gorm.DB, userID uint) error {
	return tx.Model(&models.User{}).Where("id = ?", userID).UpdateColumn("follow_count", gorm.Expr("follow_count + 1")).Error
}

func (r *followMySQLRepository) IncreaseFollowerCount(tx *gorm.DB, userID uint) error {
	return tx.Model(&models.User{}).Where("id = ?", userID).UpdateColumn("follower_count", gorm.Expr("follower_count + 1")).Error
}

func (r *followMySQLRepository) DecreaseFollowCount(tx *gorm.DB, userID uint) error {
	return tx.Model(&models.User{}).Where("id = ?", userID).UpdateColumn("follow_count", gorm.Expr("GREATEST(follow_count - 1, 0)")).Error
}

func (r *followMySQLRepository) DecreaseFollowerCount(tx *gorm.DB, userID uint) error {
	return tx.Model(&models.User{}).Where("id = ?", userID).UpdateColumn("follower_count", gorm.Expr("GREATEST(follower_count - 1, 0)")).Error
}

func (r *followMySQLRepository) ListFollowers(userID uint, page, pageSize int) ([]models.Follow, int64, error) {
	var follows []models.Follow
	var total int64
	query := r.db.Model(&models.Follow{}).Where("followed_id = ?", userID)
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	offset := (page - 1) * pageSize
	if err := query.Order("created_at DESC").Offset(offset).Limit(pageSize).Find(&follows).Error; err != nil {
		return nil, 0, err
	}
	return follows, total, nil
}

func (r *followMySQLRepository) ListFollowing(userID uint, page, pageSize int) ([]models.Follow, int64, error) {
	var follows []models.Follow
	var total int64
	query := r.db.Model(&models.Follow{}).Where("user_id = ?", userID)
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	offset := (page - 1) * pageSize
	if err := query.Order("created_at DESC").Offset(offset).Limit(pageSize).Find(&follows).Error; err != nil {
		return nil, 0, err
	}
	return follows, total, nil
}

func (r *followMySQLRepository) ListFollowingAll(userID uint) ([]models.Follow, error) {
	var follows []models.Follow
	if err := r.db.Where("user_id = ?", userID).Find(&follows).Error; err != nil {
		return nil, err
	}
	return follows, nil
}

func (r *followMySQLRepository) ListUsersByIDs(userIDs []uint) ([]models.User, error) {
	var users []models.User
	if len(userIDs) == 0 {
		return users, nil
	}
	if err := r.db.Where("id IN ?", userIDs).Find(&users).Error; err != nil {
		return nil, err
	}
	return users, nil
}

func (r *followMySQLRepository) ListRecentFeedsByUser(userID uint, limit int) ([]models.Feed, error) {
	var feeds []models.Feed
	if err := r.db.Where("user_id = ?", userID).Order("created_at DESC").Limit(limit).Find(&feeds).Error; err != nil {
		return nil, err
	}
	return feeds, nil
}

func (r *followMySQLRepository) ListFeedIDsByUser(userID uint) ([]models.Feed, error) {
	var feeds []models.Feed
	if err := r.db.Where("user_id = ?", userID).Select("id").Find(&feeds).Error; err != nil {
		return nil, err
	}
	return feeds, nil
}

func (r *followMySQLRepository) CreateTimelines(tx *gorm.DB, timelines []models.Timeline) error {
	if len(timelines) == 0 {
		return nil
	}
	return tx.Create(&timelines).Error
}

func (r *followMySQLRepository) DeleteTimelineByUserAndAuthor(userID, authorID uint) error {
	return r.db.Where("user_id = ? AND author_id = ?", userID, authorID).Delete(&models.Timeline{}).Error
}
