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
	CountByEmail(email string) (int64, error)
	Create(user *models.User) error
	GetByUsername(username string) (*models.User, error)
	GetByEmail(email string) (*models.User, error)
	GetByID(userID uint) (*models.User, error)
	IsFollowing(userID, targetUserID uint) (bool, error)
	Search(keyword string, page, pageSize int) ([]models.User, int64, error)
	ListByIDs(userIDs []uint) ([]models.User, error)
	ListPopularUsers(excludeUserID uint, page, pageSize int) ([]models.User, int64, error)
	ListTopUserIDs(limit int) ([]uint, int64, error)
	UpdateProfile(userID uint, avatar, bio, nickname *string) (*models.User, error)
	UpdatePassword(userID uint, hashedPassword string) error
	UpdateEmail(userID uint, email string) error
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

func (r *userMySQLRepository) CountByEmail(email string) (int64, error) {
	var count int64
	err := r.db.Model(&models.User{}).Where("email = ?", email).Count(&count).Error
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

func (r *userMySQLRepository) GetByEmail(email string) (*models.User, error) {
	var user models.User
	if err := r.db.Where("email = ?", email).First(&user).Error; err != nil {
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

// ListTopUserIDs 粉丝数 Top N 用户的 ID（全站排行，不排除任何人），供发现页缓存回源。
// deleted_at IS NULL 为显式声明（GORM 软删过滤本会自动追加，写出便于阅读）。
func (r *userMySQLRepository) ListTopUserIDs(limit int) ([]uint, int64, error) {
	var total int64
	if err := r.db.Model(&models.User{}).Where("deleted_at IS NULL").Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var ids []uint
	if err := r.db.Model(&models.User{}).Select("id").Where("deleted_at IS NULL").
		Order("follower_count DESC, id DESC").Limit(limit).Pluck("id", &ids).Error; err != nil {
		return nil, 0, err
	}
	return ids, total, nil
}

// ListPopularUsers 粉丝数全站排行（排除指定用户），按 follower_count DESC, id DESC 稳定排序。
// deleted_at IS NULL 为显式声明（GORM 软删过滤本会自动追加，写出便于阅读）。
func (r *userMySQLRepository) ListPopularUsers(excludeUserID uint, page, pageSize int) ([]models.User, int64, error) {
	var users []models.User
	var total int64
	query := r.db.Model(&models.User{}).Where("id <> ? AND deleted_at IS NULL", excludeUserID)
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	offset := (page - 1) * pageSize
	if err := query.Order("follower_count DESC, id DESC").Offset(offset).Limit(pageSize).Find(&users).Error; err != nil {
		return nil, 0, err
	}
	return users, total, nil
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

// UpdatePassword 更新密码并自增 token_version，一次性完成"改密即吊销旧 token"。
func (r *userMySQLRepository) UpdatePassword(userID uint, hashedPassword string) error {
	return r.db.Model(&models.User{}).Where("id = ?", userID).Updates(map[string]any{
		"password":      hashedPassword,
		"token_version": gorm.Expr("token_version + 1"),
	}).Error
}

// UpdateEmail 更新绑定邮箱并标记为已验证。
func (r *userMySQLRepository) UpdateEmail(userID uint, email string) error {
	return r.db.Model(&models.User{}).Where("id = ?", userID).Updates(map[string]any{
		"email":          email,
		"email_verified": true,
	}).Error
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
