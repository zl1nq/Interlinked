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

// RegisterRequest 邮箱验证后提交账号资料。
type RegisterRequest struct {
	RegistrationToken string `json:"registration_token" binding:"required,len=64"`
	Username          string `json:"username" binding:"required,min=3,max=50"`
	Password          string `json:"password" binding:"required,min=6,max=50"`
	Nickname          string `json:"nickname" binding:"required,min=1,max=100"`
	Email             string `json:"email" binding:"required,email,max=255"`
}

// RegisterConfirmRequest 验证邮箱并换取一次性注册凭证。
type RegisterConfirmRequest struct {
	Email string `json:"email" binding:"required,email"`
	Code  string `json:"code" binding:"required,len=6"`
}

// ChangePasswordRequest 已登录修改密码请求
type ChangePasswordRequest struct {
	OldPassword string `json:"old_password" binding:"required"`
	NewPassword string `json:"new_password" binding:"required,min=6,max=50"`
	Code        string `json:"code" binding:"required,len=6"`
}

// ResetPasswordRequest 未登录通过邮箱验证码重置密码请求
type ResetPasswordRequest struct {
	Email       string `json:"email" binding:"required,email"`
	Code        string `json:"code" binding:"required,len=6"`
	NewPassword string `json:"new_password" binding:"required,min=6,max=50"`
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

// RegisterVerify 验证邮箱，不创建账号或保存密码。
func (s *UserService) RegisterVerify(req *RegisterConfirmRequest) (string, error) {
	email := normalizeEmail(req.Email)
	if err := s.RegisterInitCheckEmail(email); err != nil {
		return "", err
	}
	cfg := config.AppConfig.Email
	return cache.VerifyRegistrationEmail(email, req.Code, cfg.CodeMaxAttempts, time.Duration(cfg.PendingTTLMin)*time.Minute)
}

// Register 在创建账号前校验并原子消费与邮箱绑定的注册凭证。
func (s *UserService) Register(req *RegisterRequest) (*models.User, error) {
	email := normalizeEmail(req.Email)
	if err := cache.CheckRegistrationTicket(req.RegistrationToken, email); err != nil {
		return nil, err
	}
	username, nickname := strings.TrimSpace(req.Username), strings.TrimSpace(req.Nickname)
	if len([]rune(username)) < 3 || nickname == "" {
		return nil, errors.New("请填写有效的用户名和昵称")
	}
	count, err := s.userRepo.CountByUsername(username)
	if err != nil {
		return nil, errors.New("查询用户失败")
	}
	if count > 0 {
		return nil, errors.New("用户名已存在")
	}
	if err := s.RegisterInitCheckEmail(email); err != nil {
		return nil, err
	}
	hashed, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, errors.New("密码加密失败")
	}
	if err := cache.ConsumeRegistrationTicket(req.RegistrationToken, email); err != nil {
		return nil, err
	}
	user := &models.User{Username: username, Nickname: nickname, Password: string(hashed), Email: &email, EmailVerified: true}
	if err := s.userRepo.Create(user); err != nil {
		// 凭证已消费，失败时要求重新验证，不能恢复凭证造成重放。
		return nil, cache.ErrRegistrationExpired
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

	token, err := utils.GenerateToken(user.ID, user.Username, user.TokenVersion)
	if err != nil {
		return nil, errors.New("生成Token失败")
	}

	return &LoginResponse{Token: token, User: user.ToResponse()}, nil
}

// SendCodeRequest 公共发送验证码请求（注册/找回密码场景）
type SendCodeRequest struct {
	Scene string `json:"scene" binding:"required"`
	Email string `json:"email" binding:"required,email"`
}

// SendChangeEmailCodeRequest 绑定/更换邮箱第一步请求。
// Password 规则：已绑定已验证邮箱的用户必填（防 session 劫持后连邮箱一起换）；
// 从未绑定过邮箱的存量用户可不传（服务端判定）。
type SendChangeEmailCodeRequest struct {
	NewEmail string `json:"new_email" binding:"required,email,max=255"`
	Password string `json:"password" binding:"omitempty,min=1,max=50"`
}

// ChangeEmailRequest 绑定/更换邮箱第二步请求。
type ChangeEmailRequest struct {
	NewEmail string `json:"new_email" binding:"required,email,max=255"`
	Code     string `json:"code" binding:"required,len=6"`
	Password string `json:"password" binding:"omitempty,min=1,max=50"`
}

// RegisterInitCheckEmail 发送、验证及完成注册前的邮箱占用预检。
func (s *UserService) RegisterInitCheckEmail(email string) error {
	email = strings.ToLower(strings.TrimSpace(email))
	count, err := s.userRepo.CountByEmail(email)
	if err != nil {
		return errors.New("查询用户失败")
	}
	if count > 0 {
		return errors.New("该邮箱已被注册")
	}
	return nil
}

// SendRegisterCode 发送或重发注册邮箱验证码。
func (s *UserService) SendRegisterCode(email string) error {
	return NewVerificationService().SendCode(EmailSceneRegister, email)
}

// SendChangePasswordCode 向当前登录用户已验证的邮箱发送修改密码验证码。
func (s *UserService) SendChangePasswordCode(userID uint) error {
	user, err := s.userRepo.GetByID(userID)
	if err != nil {
		return errors.New("用户不存在")
	}
	if user.Email == nil || !user.EmailVerified {
		return errors.New("当前账号未绑定已验证的邮箱，无法修改密码")
	}
	return NewVerificationService().SendCode(EmailSceneChangePassword, *user.Email)
}

// SendForgotPasswordCode 发送找回密码验证码。
// 邮箱不存在时静默成功，避免泄露"该邮箱是否已注册"。
func (s *UserService) SendForgotPasswordCode(email string) error {
	email = strings.ToLower(strings.TrimSpace(email))
	if _, err := s.userRepo.GetByEmail(email); err != nil {
		return nil
	}
	return NewVerificationService().SendCode(EmailSceneForgotPassword, email)
}

// ChangePassword 已登录修改密码：旧密码 + 邮箱验证码双重校验。
// 成功后 token_version +1，吊销所有已签发 token（含当前设备）。
func (s *UserService) ChangePassword(userID uint, req *ChangePasswordRequest) error {
	user, err := s.userRepo.GetByID(userID)
	if err != nil {
		return errors.New("用户不存在")
	}
	if user.Email == nil || !user.EmailVerified {
		return errors.New("当前账号未绑定已验证的邮箱，无法修改密码")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.OldPassword)); err != nil {
		return errors.New("旧密码错误")
	}
	if err := NewVerificationService().VerifyCode(EmailSceneChangePassword, *user.Email, req.Code); err != nil {
		return err
	}

	hashed, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		return errors.New("密码加密失败")
	}
	if err := s.userRepo.UpdatePassword(userID, string(hashed)); err != nil {
		return errors.New("修改密码失败")
	}
	cache.InvalidateTokenVersion(userID)
	return nil
}

// ResetPassword 未登录重置密码：仅凭邮箱验证码。// 成功后 token_version +1，盗号者持有的旧 token 一并失效。
func (s *UserService) ResetPassword(req *ResetPasswordRequest) error {
	email := strings.ToLower(strings.TrimSpace(req.Email))

	if err := NewVerificationService().VerifyCode(EmailSceneForgotPassword, email, req.Code); err != nil {
		return err
	}

	user, err := s.userRepo.GetByEmail(email)
	if err != nil {
		return errors.New("该邮箱未注册")
	}

	hashed, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		return errors.New("密码加密失败")
	}
	if err := s.userRepo.UpdatePassword(user.ID, string(hashed)); err != nil {
		return errors.New("重置密码失败")
	}
	cache.InvalidateTokenVersion(user.ID)
	return nil
}

// SendChangeEmailCode 绑定/更换邮箱第一步：校验新邮箱可用性，
// 对已绑定邮箱的用户额外验证当前密码，然后向新邮箱发送验证码。
func (s *UserService) SendChangeEmailCode(userID uint, req *SendChangeEmailCodeRequest) error {
	user, err := s.userRepo.GetByID(userID)
	if err != nil {
		return errors.New("用户不存在")
	}

	newEmail := strings.ToLower(strings.TrimSpace(req.NewEmail))
	if user.Email != nil && strings.EqualFold(*user.Email, newEmail) {
		return errors.New("新邮箱与当前邮箱相同")
	}
	// 发码时先查重，避免用户验证完码才发现邮箱被占用
	if err := s.checkEmailAvailable(newEmail); err != nil {
		return err
	}

	// 只有"已绑定已验证邮箱"的账号才要求密码；存量未绑定用户走纯绑定流程
	if user.Email != nil && user.EmailVerified {
		if req.Password == "" {
			return errors.New("请输入当前密码")
		}
		if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); err != nil {
			return errors.New("密码错误")
		}
	}

	return NewVerificationService().SendCode(EmailSceneChangeEmail, newEmail)
}

// ChangeEmail 绑定/更换邮箱第二步：校验密码（如需要）与验证码，
// 落库前再次查重防并发窗口，成功后立即覆盖旧邮箱并标记已验证。
func (s *UserService) ChangeEmail(userID uint, req *ChangeEmailRequest) error {
	user, err := s.userRepo.GetByID(userID)
	if err != nil {
		return errors.New("用户不存在")
	}

	newEmail := strings.ToLower(strings.TrimSpace(req.NewEmail))
	if user.Email != nil && strings.EqualFold(*user.Email, newEmail) {
		return errors.New("新邮箱与当前邮箱相同")
	}
	if user.Email != nil && user.EmailVerified {
		if req.Password == "" {
			return errors.New("请输入当前密码")
		}
		if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); err != nil {
			return errors.New("密码错误")
		}
	}
	if err := NewVerificationService().VerifyCode(EmailSceneChangeEmail, newEmail, req.Code); err != nil {
		return err
	}
	if err := s.checkEmailAvailable(newEmail); err != nil {
		return err
	}

	if err := s.userRepo.UpdateEmail(userID, newEmail); err != nil {
		return errors.New("绑定邮箱失败")
	}
	cache.DeleteUserCacheWithRetry(userID, 24*time.Hour)
	return nil
}

// checkEmailAvailable 校验邮箱未被其他账号占用。
func (s *UserService) checkEmailAvailable(email string) error {
	count, err := s.userRepo.CountByEmail(email)
	if err != nil {
		return errors.New("查询用户失败")
	}
	if count > 0 {
		return errors.New("该邮箱已被其他账号绑定")
	}
	return nil
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
