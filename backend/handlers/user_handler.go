package handlers

import (
	"feed/middleware"
	"feed/services"
	"feed/utils"
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

// Register 用户注册
// POST /api/auth/register
func (h *UserHandler) Register(c *gin.Context) {
	var req services.RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, 400, "参数错误: "+err.Error())
		return
	}

	user, err := h.userService.Register(&req)
	if err != nil {
		utils.Error(c, 400, err.Error())
		return
	}

	// 自动登录，返回Token
	token, err := utils.GenerateToken(user.ID, user.Username)
	if err != nil {
		utils.Error(c, 500, "生成Token失败")
		return
	}

	utils.Success(c, gin.H{
		"token": token,
		"user":  user.ToResponse(),
	})
}

// Login 用户登录
// POST /api/auth/login
func (h *UserHandler) Login(c *gin.Context) {
	var req services.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, 400, "参数错误: "+err.Error())
		return
	}

	resp, err := h.userService.Login(&req)
	if err != nil {
		utils.Error(c, 400, err.Error())
		return
	}

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
