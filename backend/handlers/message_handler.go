package handlers

import (
	"feed/middleware"
	"feed/services"
	"feed/utils"
	"strconv"

	"github.com/gin-gonic/gin"
)

// MessageHandler 提供私信 HTTP 兜底接口，避免历史和会话完全依赖 WebSocket。
type MessageHandler struct {
	messageService *services.MessageService
}

func NewMessageHandler(messageService *services.MessageService) *MessageHandler {
	return &MessageHandler{messageService: messageService}
}

func (h *MessageHandler) ListConversations(c *gin.Context) {
	currentUserID := middleware.GetCurrentUserID(c)
	page := parsePositiveInt(c.Query("page"), 1)
	pageSize := parsePositiveInt(c.Query("page_size"), 50)
	if pageSize > 50 {
		pageSize = 50
	}

	list, total, err := h.messageService.GetConversationList(currentUserID, page, pageSize)
	if err != nil {
		utils.Error(c, 500, err.Error())
		return
	}
	utils.Success(c, gin.H{"list": list, "total": total, "page": page, "page_size": pageSize})
}

func (h *MessageHandler) GetHistory(c *gin.Context) {
	currentUserID := middleware.GetCurrentUserID(c)
	targetID64, err := strconv.ParseUint(c.Param("target_id"), 10, 64)
	if err != nil || targetID64 == 0 {
		utils.Error(c, 400, "target_user_id 无效")
		return
	}
	page := parsePositiveInt(c.Query("page"), 1)
	pageSize := parsePositiveInt(c.Query("page_size"), 100)
	if pageSize > 100 {
		pageSize = 100
	}

	list, total, err := h.messageService.GetConversationMessages(currentUserID, uint(targetID64), page, pageSize)
	if err != nil {
		utils.Error(c, 500, err.Error())
		return
	}
	utils.Success(c, gin.H{"target_user_id": uint(targetID64), "list": list, "total": total, "page": page, "page_size": pageSize})
}

func parsePositiveInt(raw string, fallback int) int {
	value, err := strconv.Atoi(raw)
	if err != nil || value < 1 {
		return fallback
	}
	return value
}
