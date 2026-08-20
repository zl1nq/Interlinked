package handlers

import (
	"feed/middleware"
	"feed/services"
	"feed/utils"
	"strconv"

	"github.com/gin-gonic/gin"
)

// FollowHandler 负责关注关系域 HTTP 入口：关注、取关、粉丝/关注列表。
type FollowHandler struct {
	followService *services.FollowService
}

func NewFollowHandler(followService *services.FollowService) *FollowHandler {
	return &FollowHandler{followService: followService}
}

// Follow 关注用户
// POST /api/follow/:id
func (h *FollowHandler) Follow(c *gin.Context) {
	currentUserID := middleware.GetCurrentUserID(c)
	targetIDStr := c.Param("id")
	targetID, err := strconv.ParseUint(targetIDStr, 10, 64)
	if err != nil {
		utils.Error(c, 400, "用户ID无效")
		return
	}

	if err := h.followService.Follow(currentUserID, uint(targetID)); err != nil {
		utils.Error(c, 400, err.Error())
		return
	}

	utils.SuccessWithMessage(c, "关注成功", nil)
}

// Unfollow 取消关注
// DELETE /api/follow/:id
func (h *FollowHandler) Unfollow(c *gin.Context) {
	currentUserID := middleware.GetCurrentUserID(c)
	targetIDStr := c.Param("id")
	targetID, err := strconv.ParseUint(targetIDStr, 10, 64)
	if err != nil {
		utils.Error(c, 400, "用户ID无效")
		return
	}

	if err := h.followService.Unfollow(currentUserID, uint(targetID)); err != nil {
		utils.Error(c, 400, err.Error())
		return
	}

	utils.SuccessWithMessage(c, "取消关注成功", nil)
}

// GetFollowers 获取粉丝列表
// GET /api/users/:id/followers?page=1&page_size=20
func (h *FollowHandler) GetFollowers(c *gin.Context) {
	userIDStr := c.Param("id")
	userID, err := strconv.ParseUint(userIDStr, 10, 64)
	if err != nil {
		utils.Error(c, 400, "用户ID无效")
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

	users, total, err := h.followService.GetFollowers(uint(userID), page, pageSize)
	if err != nil {
		utils.Error(c, 500, "获取粉丝列表失败")
		return
	}

	utils.SuccessPage(c, users, total, page, pageSize)
}

// GetFollowing 获取关注列表
// GET /api/users/:id/following?page=1&page_size=20
func (h *FollowHandler) GetFollowing(c *gin.Context) {
	userIDStr := c.Param("id")
	userID, err := strconv.ParseUint(userIDStr, 10, 64)
	if err != nil {
		utils.Error(c, 400, "用户ID无效")
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

	users, total, err := h.followService.GetFollowing(uint(userID), page, pageSize)
	if err != nil {
		utils.Error(c, 500, "获取关注列表失败")
		return
	}

	utils.SuccessPage(c, users, total, page, pageSize)
}
