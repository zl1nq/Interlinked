package handlers

import (
	"feed/middleware"
	"feed/services"
	"feed/utils"
	"fmt"
	"strconv"

	"github.com/gin-gonic/gin"
)

// UserHandler 负责用户域 HTTP 入口：注册、登录、资料查询与更新。
type UserHandler struct {
	userService *services.UserService
}

func NewUserHandler(userService *services.UserService) *UserHandler {
	return &UserHandler{userService: userService}
}

// Register 注册第一步：提交资料并发送邮箱验证码（不建号）
// POST /api/auth/register
func (h *UserHandler) Register(c *gin.Context) {
	var req services.RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, 400, "参数错误: "+err.Error())
		return
	}

	if err := h.userService.RegisterInit(&req); err != nil {
		utils.Error(c, 400, err.Error())
		return
	}

	utils.SuccessWithMessage(c, "验证码已发送，请查收邮箱完成注册", nil)
}

// RegisterConfirm 注册第二步：提交邮箱验证码完成建号，成功后自动登录
// POST /api/auth/register/confirm
func (h *UserHandler) RegisterConfirm(c *gin.Context) {
	var req services.RegisterConfirmRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, 400, "参数错误: "+err.Error())
		return
	}

	user, err := h.userService.RegisterConfirm(&req)
	if err != nil {
		utils.Error(c, 400, err.Error())
		return
	}

	// 自动登录，返回Token
	token, err := utils.GenerateToken(user.ID, user.Username, user.TokenVersion)
	if err != nil {
		utils.Error(c, 500, "生成Token失败")
		return
	}

	utils.Success(c, gin.H{
		"token": token,
		"user":  user.ToResponse(),
	})
}

// SendEmailCode 发送邮箱验证码（注册/找回密码场景，无需登录）
// POST /api/auth/email/code
func (h *UserHandler) SendEmailCode(c *gin.Context) {
	var req services.SendCodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, 400, "参数错误: "+err.Error())
		return
	}

	switch req.Scene {
	case services.EmailSceneRegister:
		if err := h.userService.RegisterInitCheckEmail(req.Email); err != nil {
			utils.Error(c, 400, err.Error())
			return
		}
		if err := h.userService.SendRegisterCode(req.Email); err != nil {
			utils.Error(c, 400, err.Error())
			return
		}
	case services.EmailSceneForgotPassword:
		if err := h.userService.SendForgotPasswordCode(req.Email); err != nil {
			utils.Error(c, 400, err.Error())
			return
		}
	default:
		utils.Error(c, 400, "不支持的验证码场景")
		return
	}

	utils.SuccessWithMessage(c, "验证码已发送，请查收邮箱", nil)
}

// SendChangeEmailCode 绑定/更换邮箱第一步：向新邮箱发送验证码（需登录）
// POST /users/me/email/change/code
func (h *UserHandler) SendChangeEmailCode(c *gin.Context) {
	userID := middleware.GetCurrentUserID(c)
	if userID == 0 {
		utils.Unauthorized(c, "请先登录")
		return
	}

	var req services.SendChangeEmailCodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, 400, "参数错误: "+err.Error())
		return
	}

	if err := h.userService.SendChangeEmailCode(userID, &req); err != nil {
		utils.Error(c, 400, err.Error())
		return
	}

	utils.SuccessWithMessage(c, "验证码已发送，请查收新邮箱", nil)
}

// ChangeEmail 绑定/更换邮箱第二步：验证码确认后立即生效（需登录）
// PUT /users/me/email
func (h *UserHandler) ChangeEmail(c *gin.Context) {
	userID := middleware.GetCurrentUserID(c)
	if userID == 0 {
		utils.Unauthorized(c, "请先登录")
		return
	}

	var req services.ChangeEmailRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, 400, "参数错误: "+err.Error())
		return
	}

	if err := h.userService.ChangeEmail(userID, &req); err != nil {
		utils.Error(c, 400, err.Error())
		return
	}

	user, err := h.userService.GetUserByID(userID)
	if err != nil {
		utils.SuccessWithMessage(c, "邮箱绑定成功", nil)
		return
	}

	utils.SuccessWithMessage(c, "邮箱绑定成功", user.ToResponse())
}

// SendChangePasswordCode 发送修改密码验证码（需登录，发送到当前用户邮箱）
// POST /api/users/me/password/code
func (h *UserHandler) SendChangePasswordCode(c *gin.Context) {
	userID := middleware.GetCurrentUserID(c)
	if userID == 0 {
		utils.Unauthorized(c, "请先登录")
		return
	}

	if err := h.userService.SendChangePasswordCode(userID); err != nil {
		utils.Error(c, 400, err.Error())
		return
	}

	utils.SuccessWithMessage(c, "验证码已发送，请查收邮箱", nil)
}

// ChangePassword 已登录修改密码（旧密码 + 邮箱验证码）
// PUT /api/users/me/password
func (h *UserHandler) ChangePassword(c *gin.Context) {
	userID := middleware.GetCurrentUserID(c)
	if userID == 0 {
		utils.Unauthorized(c, "请先登录")
		return
	}

	var req services.ChangePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, 400, "参数错误: "+err.Error())
		return
	}

	if err := h.userService.ChangePassword(userID, &req); err != nil {
		utils.Error(c, 400, err.Error())
		return
	}

	utils.SuccessWithMessage(c, "密码修改成功，请重新登录", nil)
}

// ResetPassword 未登录重置密码（邮箱验证码）
// POST /api/auth/password/reset
func (h *UserHandler) ResetPassword(c *gin.Context) {
	var req services.ResetPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, 400, "参数错误: "+err.Error())
		return
	}

	if err := h.userService.ResetPassword(&req); err != nil {
		utils.Error(c, 400, err.Error())
		return
	}

	utils.SuccessWithMessage(c, "密码重置成功，请使用新密码登录", nil)
}

// Login 用户登录
// POST /api/auth/login
func (h *UserHandler) Login(c *gin.Context) {
	var req services.LoginRequest
	//从 JSON Body 解析登录参数
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, 400, "参数错误: "+err.Error())
		return
	}
	utils.LogInfo(fmt.Sprintf("用户 %s 尝试登录 (IP: %s)", req.Username, c.ClientIP()))
	resp, err := h.userService.Login(&req)
	if err != nil {
		utils.Error(c, 400, err.Error())
		return
	}

	utils.LogInfo(fmt.Sprintf("用户 %s 登录成功", req.Username))
	utils.Success(c, resp)
}

// GetProfile 获取用户资料
// GET /api/users/:id
func (h *UserHandler) GetProfile(c *gin.Context) {
	userIDStr := c.Param("id")
	userID, err := strconv.ParseUint(userIDStr, 10, 64)
	if err != nil {
		utils.Error(c, 400, "用户ID无效")
		return
	}

	currentUserID := middleware.GetCurrentUserID(c)

	profile, err := h.userService.GetUserProfile(uint(userID), currentUserID)
	if err != nil {
		utils.Error(c, 404, err.Error())
		return
	}

	_ = h.userService.RecordVisit(currentUserID, uint(userID))

	utils.Success(c, profile)
}

// GetCurrentUser 获取当前登录用户信息
// GET /api/users/me
func (h *UserHandler) GetCurrentUser(c *gin.Context) {
	userID := middleware.GetCurrentUserID(c)
	if userID == 0 {
		utils.Unauthorized(c, "请先登录")
		return
	}

	user, err := h.userService.GetUserByID(userID)
	if err != nil {
		utils.Error(c, 404, err.Error())
		return
	}

	utils.Success(c, user.ToResponse())
}

// SearchUsers 搜索用户
// GET /api/users/search?keyword=xxx&page=1&page_size=20
func (h *UserHandler) SearchUsers(c *gin.Context) {
	keyword := c.Query("keyword")
	if keyword == "" {
		utils.Error(c, 400, "关键词不能为空")
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 50 {
		pageSize = 20
	}

	users, total, err := h.userService.SearchUsers(keyword, page, pageSize)
	if err != nil {
		utils.Error(c, 500, "搜索失败")
		return
	}

	utils.SuccessPage(c, users, total, page, pageSize)
}

// UpdateProfile 更新当前登录用户资料
// PUT /api/users/me
func (h *UserHandler) UpdateProfile(c *gin.Context) {
	userID := middleware.GetCurrentUserID(c)
	if userID == 0 {
		utils.Unauthorized(c, "请先登录")
		return
	}

	var req services.UpdateProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, 400, "参数错误: "+err.Error())
		return
	}

	user, err := h.userService.UpdateProfile(userID, &req)
	if err != nil {
		utils.Error(c, 500, err.Error())
		return
	}

	utils.Success(c, user.ToResponse())
}

// GetRecentVisits 获取最近访客
// GET /api/users/me/visits?page=1&page_size=20
func (h *UserHandler) GetRecentVisits(c *gin.Context) {
	userID := middleware.GetCurrentUserID(c)
	if userID == 0 {
		utils.Unauthorized(c, "请先登录")
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 50 {
		pageSize = 20
	}

	list, total, err := h.userService.GetRecentVisits(userID, page, pageSize)
	if err != nil {
		utils.Error(c, 500, "获取访客列表失败")
		return
	}

	utils.SuccessPage(c, list, total, page, pageSize)
}
