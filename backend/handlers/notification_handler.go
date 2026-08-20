package handlers

import (
	"feed/middleware"
	"feed/services"
	"feed/utils"
	"strconv"

	"github.com/gin-gonic/gin"
)

// NotificationHandler 提供通知中心接口。
type NotificationHandler struct {
	notificationService *services.NotificationService
}

func NewNotificationHandler(notificationService *services.NotificationService) *NotificationHandler {
	return &NotificationHandler{notificationService: notificationService}
}

// ListNotifications 获取通知列表。
// GET /api/notifications?page=1&page_size=20
func (h *NotificationHandler) ListNotifications(c *gin.Context) {
	userID := middleware.GetCurrentUserID(c)
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 50 {
		pageSize = 20
	}

	list, total, unread, err := h.notificationService.ListNotifications(userID, page, pageSize) //获取通知列表
	if err != nil {
		utils.Error(c, 500, "获取通知失败")
		return
	}

	utils.Success(c, gin.H{
		"list":         list,
		"total":        total,
		"page":         page,
		"page_size":    pageSize,
		"has_more":     int64(page*pageSize) < total,
		"unread_count": unread,
	})
}

// MarkAllRead 全部标记已读。
// POST /api/notifications/read-all
func (h *NotificationHandler) MarkAllRead(c *gin.Context) {
	userID := middleware.GetCurrentUserID(c)
	if err := h.notificationService.MarkAllRead(userID); err != nil { //标记已读
		utils.Error(c, 500, "标记已读失败")
		return
	}
	utils.SuccessWithMessage(c, "已全部标记已读", nil)
}
