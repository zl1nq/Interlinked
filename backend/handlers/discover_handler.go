package handlers

import (
	"feed/middleware"
	"feed/services"
	"feed/utils"
	"strconv"

	"github.com/gin-gonic/gin"
)

// DiscoverHandler 提供发现页热门推荐接口。
type DiscoverHandler struct {
	discoverService *services.DiscoverService
}

func NewDiscoverHandler(discoverService *services.DiscoverService) *DiscoverHandler {
	return &DiscoverHandler{discoverService: discoverService}
}

// PopularUsers 热门用户列表。
// GET /api/discover/popular-users?page=1&page_size=10
func (h *DiscoverHandler) PopularUsers(c *gin.Context) {
	currentUserID := middleware.GetCurrentUserID(c)
	page, pageSize := parseDiscoverPage(c)

	list, total, err := h.discoverService.PopularUsers(currentUserID, page, pageSize)
	if err != nil {
		utils.Error(c, 500, "获取热门用户失败")
		return
	}

	utils.Success(c, gin.H{
		"list":     list,
		"total":    total,
		"has_more": int64(page*pageSize) < total,
	})
}

// PopularFeeds 热门动态列表。
// GET /api/discover/popular-feeds?page=1&page_size=10
func (h *DiscoverHandler) PopularFeeds(c *gin.Context) {
	currentUserID := middleware.GetCurrentUserID(c)
	page, pageSize := parseDiscoverPage(c)

	list, total, err := h.discoverService.PopularFeeds(currentUserID, page, pageSize)
	if err != nil {
		utils.Error(c, 500, "获取热门动态失败")
		return
	}

	utils.Success(c, gin.H{
		"list":     list,
		"total":    total,
		"has_more": int64(page*pageSize) < total,
	})
}

// parseDiscoverPage 解析分页参数：默认 10 条/页，上限 50。
func parseDiscoverPage(c *gin.Context) (page, pageSize int) {
	page, _ = strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ = strconv.Atoi(c.DefaultQuery("page_size", "10"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 50 {
		pageSize = 10
	}
	return page, pageSize
}
