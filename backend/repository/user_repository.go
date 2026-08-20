package repository

import (
	"feed/models"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// UserRepository 定义用户域持久化契约。
// 约定：service 只依赖接口，不关心具体 ORM 实现。
type UserRepository interface {
	CountByUsername(username string) (int64, error)
	Create(user *models.User) error
	GetByUsername(username string) (*models.User, error)
	GetByID(userID uint) (*models.User, error)
	IsFollowing(userID, targetUserID uint) (bool, error)
	Search(keyword string, page, pageSize int) ([]models.User, int64, error)
	ListByIDs(userIDs []uint) ([]models.User, error)
	UpdateProfile(userID uint, avatar, bio, nickname *string) (*models.User, error)
	UpdateBigV(userID uint, isBigV bool) error
	UpsertVisit(visitorID, targetUserID uint, visitedAt time.Time) error
	ListRecentVisits(targetUserID uint, page, pageSize int) ([]models.Visit, int64, error)
}

type userMySQLRepository struct {
	db *gorm.DB
}

func NewUserRepository(db *gorm.DB) UserRepository {
	return &userMySQLRepository{db: db}
}

func (r *userMySQLRepository) CountByUsername(username string) (int64, error) {
	var count int64
	err := r.db.Model(&models.User{}).Where("username = ?", username).Count(&count).Error
	return count, err
}

func (r *userMySQLRepository) Create(user *models.User) error {
	return r.db.Create(user).Error
}

func (r *userMySQLRepository) GetByUsername(username string) (*models.User, error) {
	var user models.User
	if err := r.db.Where("username = ?", username).First(&user).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *userMySQLRepository) GetByID(userID uint) (*models.User, error) {
	var user models.User
	if err := r.db.Where("id = ?", userID).First(&user).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *userMySQLRepository) IsFollowing(userID, targetUserID uint) (bool, error) {
	var count int64
	err := r.db.Model(&models.Follow{}).Where("user_id = ? AND followed_id = ?", userID, targetUserID).Count(&count).Error
	return count > 0, err
}

func (r *userMySQLRepository) Search(keyword string, page, pageSize int) ([]models.User, int64, error) {
	var users []models.User
	var total int64

	// Phase1 搜索优化：优先使用前缀匹配，命中 username/nickname 索引，避免全表扫描。
	query := r.db.Model(&models.User{}).Where("username LIKE ? OR nickname LIKE ?", keyword+"%", keyword+"%")
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize
	if err := query.Offset(offset).Limit(pageSize).Find(&users).Error; err != nil {
		return nil, 0, err
	}

	return users, total, nil
}

func (r *userMySQLRepository) ListByIDs(userIDs []uint) ([]models.User, error) {
	var users []models.User
	if len(userIDs) == 0 {
		return users, nil
	}
	if err := r.db.Where("id IN ?", userIDs).Find(&users).Error; err != nil {
		return nil, err
	}
	return users, nil
}

func (r *userMySQLRepository) UpdateProfile(userID uint, avatar, bio, nickname *string) (*models.User, error) {
	updates := map[string]any{}
	if avatar != nil {
		updates["avatar"] = *avatar
	}
	if bio != nil {
		updates["bio"] = *bio
	}
	if nickname != nil {
		updates["nickname"] = *nickname
	}

	if len(updates) > 0 {
		if err := r.db.Model(&models.User{}).Where("id = ?", userID).Updates(updates).Error; err != nil {
			return nil, err
		}
	}

	return r.GetByID(userID)
}

func (r *userMySQLRepository) UpdateBigV(userID uint, isBigV bool) error {
	return r.db.Model(&models.User{}).Where("id = ?", userID).Update("is_big_v", isBigV).Error
}

func (r *userMySQLRepository) UpsertVisit(visitorID, targetUserID uint, visitedAt time.Time) error {
	visit := models.Visit{
		VisitorID:    visitorID,
		TargetUserID: targetUserID,
		VisitedAt:    visitedAt,
	}
	return r.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "visitor_id"}, {Name: "target_user_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"visited_at", "updated_at"}),
	}).Create(&visit).Error
}

func (r *userMySQLRepository) ListRecentVisits(targetUserID uint, page, pageSize int) ([]models.Visit, int64, error) {
	var visits []models.Visit
	var total int64

	query := r.db.Model(&models.Visit{}).Where("target_user_id = ?", targetUserID)
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	offset := (page - 1) * pageSize
	if err := query.Preload("Visitor").Order("visited_at DESC").Offset(offset).Limit(pageSize).Find(&visits).Error; err != nil {
		return nil, 0, err
	}

	return visits, total, nil
}
