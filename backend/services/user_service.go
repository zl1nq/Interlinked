package services

import (
	"encoding/json"
	"errors"
	"feed/cache"
	"feed/config"
	"feed/models"
	"feed/repository"
	"feed/utils"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// UserService 负责用户域业务编排。
// 说明：
// - 认证、资料、搜索、访客等业务规则在此层实现；
// - 底层数据访问通过 UserRepository 完成。
type UserService struct {
	userRepo repository.UserRepository
}

func NewUserService() *UserService {
	return &UserService{userRepo: repository.NewUserRepository(models.DB)}
}

// RegisterRequest 注册请求
type RegisterRequest struct {
	Username string `json:"username" binding:"required,min=3,max=50"`
	Password string `json:"password" binding:"required,min=6,max=50"`
	Nickname string `json:"nickname" binding:"required,min=1,max=100"`
}

// LoginRequest 登录请求
type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// LoginResponse 登录响应
type LoginResponse struct {
	Token string              `json:"token"`
	User  models.UserResponse `json:"user"`
}

// UpdateProfileRequest 更新个人资料请求
type UpdateProfileRequest struct {
	Avatar   *string `json:"avatar" binding:"omitempty,max=500"`
	Bio      *string `json:"bio" binding:"omitempty,max=500"`
	Nickname *string `json:"nickname" binding:"omitempty,min=1,max=100"`
}

type VisitResponse struct {
	ID        uint                `json:"id"`
	VisitedAt string              `json:"visited_at"`
	Visitor   models.UserResponse `json:"visitor"`
}

// Register 用户注册
func (s *UserService) Register(req *RegisterRequest) (*models.User, error) {
	count, err := s.userRepo.CountByUsername(req.Username)
	if err != nil {
		return nil, errors.New("查询用户失败")
	}
	if count > 0 {
		return nil, errors.New("用户名已存在")
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, errors.New("密码加密失败")
	}

	user := &models.User{
		Username: req.Username,
		Password: string(hashedPassword),
		Nickname: req.Nickname,
	}

	if err := s.userRepo.Create(user); err != nil {
		return nil, errors.New("注册失败")
	}
	cache.AddUserID(user.ID)

	return user, nil
}

// Login 用户登录
func (s *UserService) Login(req *LoginRequest) (*LoginResponse, error) {
	user, err := s.userRepo.GetByUsername(req.Username)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("用户名或密码错误")
		}
		return nil, errors.New("查询用户失败")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); err != nil {
		return nil, errors.New("用户名或密码错误")
	}

	token, err := utils.GenerateToken(user.ID, user.Username)
	if err != nil {
		return nil, errors.New("生成Token失败")
	}

	return &LoginResponse{Token: token, User: user.ToResponse()}, nil
}

// GetUserByID 根据ID获取用户
func (s *UserService) GetUserByID(userID uint) (*models.User, error) {
	user, err := s.userRepo.GetByID(userID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("用户不存在")
		}
		return nil, err
	}
	return user, nil
}

// GetUserProfile 获取用户资料（含是否关注状态）
func (s *UserService) GetUserProfile(targetUserID, currentUserID uint) (*models.UserResponse, error) {
	if !cache.MightUserExist(targetUserID) {
		return nil, errors.New("用户不存在")
	}

	var resp models.UserResponse
	if raw, err := cache.GetUserInfo(targetUserID); err == nil && raw != "" {
		if unmarshalErr := json.Unmarshal([]byte(raw), &resp); unmarshalErr == nil {
			if currentUserID > 0 && currentUserID != targetUserID {
				isFollowed, _ := cache.IsFollowing(currentUserID, targetUserID)
				if !isFollowed {
					isFollowed, _ = s.userRepo.IsFollowing(currentUserID, targetUserID)
				}
				resp.IsFollowed = isFollowed
			}
			return &resp, nil
		}
	}

	lockKey := "lock:user_profile:" + strconv.FormatUint(uint64(targetUserID), 10)
	lockToken := strconv.FormatInt(time.Now().UnixMilli(), 10)
	locked, _ := cache.AcquireLock(lockKey, lockToken, 3*time.Second)
	if !locked {
		for range 8 {
			time.Sleep(40 * time.Millisecond)
			if raw, err := cache.GetUserInfo(targetUserID); err == nil && raw != "" {
				if unmarshalErr := json.Unmarshal([]byte(raw), &resp); unmarshalErr == nil {
					if currentUserID > 0 && currentUserID != targetUserID {
						isFollowed, _ := cache.IsFollowing(currentUserID, targetUserID)
						if !isFollowed {
							isFollowed, _ = s.userRepo.IsFollowing(currentUserID, targetUserID)
						}
						resp.IsFollowed = isFollowed
					}
					return &resp, nil
				}
			}
		}
	}
	if locked {
		defer func() { _ = cache.ReleaseLock(lockKey, lockToken) }()
	}

	user, dbErr := s.GetUserByID(targetUserID)
	if dbErr != nil {
		return nil, dbErr
	}
	resp = user.ToResponse()
	if b, mErr := json.Marshal(resp); mErr == nil {
		_ = cache.CacheUserInfo(targetUserID, string(b))
	}

	if currentUserID > 0 && currentUserID != targetUserID {
		isFollowed, _ := cache.IsFollowing(currentUserID, targetUserID)
		if !isFollowed {
			isFollowed, _ = s.userRepo.IsFollowing(currentUserID, targetUserID)
		}
		resp.IsFollowed = isFollowed
	}

	return &resp, nil
}

// UpdateBigVStatus 更新用户大V状态
func (s *UserService) UpdateBigVStatus(userID uint) error {
	user, err := s.userRepo.GetByID(userID)
	if err != nil {
		return err
	}

	threshold := int64(config.AppConfig.Feed.BigVThreshold)
	isBigV := user.FollowerCount >= threshold

	if user.IsBigV != isBigV {
		if err := s.userRepo.UpdateBigV(userID, isBigV); err != nil {
			return err
		}
		cache.SetBigV(userID, isBigV)
		cache.DeleteUserCacheWithRetry(userID, 24*time.Hour)
	}

	return nil
}

// SearchUsers 搜索用户
func (s *UserService) SearchUsers(keyword string, page, pageSize int) ([]models.UserResponse, int64, error) {
	users, total, err := s.userRepo.Search(keyword, page, pageSize)
	if err != nil {
		return nil, 0, err
	}

	responses := make([]models.UserResponse, 0, len(users))
	for _, user := range users {
		responses = append(responses, user.ToResponse())
	}
	return responses, total, nil
}

// UpdateProfile 更新个人资料
func (s *UserService) UpdateProfile(userID uint, req *UpdateProfileRequest) (*models.User, error) {
	if req.Nickname != nil {
		nickname := strings.TrimSpace(*req.Nickname)
		if nickname == "" {
			return nil, errors.New("昵称不能为空")
		}
		req.Nickname = &nickname
	}

	if req.Bio != nil {
		bio := strings.TrimSpace(*req.Bio)
		req.Bio = &bio
	}

	user, err := s.userRepo.UpdateProfile(userID, req.Avatar, req.Bio, req.Nickname)
	if err != nil {
		return nil, errors.New("更新个人资料失败")
	}

	cache.DeleteUserCache(userID)

	return user, nil
}

// RecordVisit 记录主页访问
func (s *UserService) RecordVisit(visitorID, targetUserID uint) error {
	if visitorID == 0 || targetUserID == 0 || visitorID == targetUserID {
		return nil
	}
	return s.userRepo.UpsertVisit(visitorID, targetUserID, time.Now())
}

// GetRecentVisits 获取最近访问记录
func (s *UserService) GetRecentVisits(targetUserID uint, page, pageSize int) ([]VisitResponse, int64, error) {
	visits, total, err := s.userRepo.ListRecentVisits(targetUserID, page, pageSize)
	if err != nil {
		return nil, 0, err
	}

	result := make([]VisitResponse, 0, len(visits))
	for _, v := range visits {
		result = append(result, VisitResponse{
			ID:        v.ID,
			VisitedAt: v.VisitedAt.Format(time.RFC3339),
			Visitor:   v.Visitor.ToResponse(),
		})
	}
	return result, total, nil
}
