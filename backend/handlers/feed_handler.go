package handlers

import (
	"feed/middleware"
	"feed/services"
	"feed/utils"
	"fmt"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

// FeedHandler 负责动态域 HTTP 入口：发布、时间线、互动（赞评转）等。
type FeedHandler struct {
	feedService *services.FeedService
}

func NewFeedHandler(feedService *services.FeedService) *FeedHandler {
	return &FeedHandler{feedService: feedService}
}

// PublishFeed 发布动态（支持文案+图片+视频）。
// 输入：CreateFeedRequest（content/images/videos）
// 输出：完整 FeedResponse（含作者与互动状态）。
// POST /api/feeds
func (h *FeedHandler) PublishFeed(c *gin.Context) {
	currentUserID := middleware.GetCurrentUserID(c)
	utils.LogInfo(fmt.Sprintf("currentUserID: %d is publishing feed", currentUserID))
	var req services.CreateFeedRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, 400, "参数错误: "+err.Error())
		return
	}

	feed, err := h.feedService.PublishFeed(currentUserID, &req)
	if err != nil {
		utils.Error(c, 500, err.Error())
		return
	}

	resp, _ := h.feedService.GetFeedByID(feed.ID, currentUserID)
	utils.SuccessWithMessage(c, "发布成功", resp)
}

// UpdateFeed 编辑动态
// PUT /api/feeds/:id
func (h *FeedHandler) UpdateFeed(c *gin.Context) {
	currentUserID := middleware.GetCurrentUserID(c)
	feedIDStr := c.Param("id")
	feedID, err := strconv.ParseUint(feedIDStr, 10, 64)
	if err != nil {
		utils.Error(c, 400, "动态ID无效")
		return
	}

	var req services.UpdateFeedRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, 400, "参数错误: "+err.Error())
		return
	}

	_, err = h.feedService.UpdateFeed(uint(feedID), currentUserID, &req)
	if err != nil {
		utils.Error(c, 400, err.Error())
		return
	}

	resp, _ := h.feedService.GetFeedByID(uint(feedID), currentUserID)
	utils.SuccessWithMessage(c, "编辑成功", resp)
}

// RepostFeed 转发动态
// POST /api/feeds/repost
func (h *FeedHandler) RepostFeed(c *gin.Context) {
	currentUserID := middleware.GetCurrentUserID(c)

	var req services.RepostFeedRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, 400, "参数错误: "+err.Error())
		return
	}

	feed, err := h.feedService.RepostFeed(currentUserID, &req)
	if err != nil {
		utils.Error(c, 400, err.Error())
		return
	}

	resp, _ := h.feedService.GetFeedByID(feed.ID, currentUserID)
	utils.SuccessWithMessage(c, "转发成功", resp)
}

// DeleteFeed 删除动态
// DELETE /api/feeds/:id
func (h *FeedHandler) DeleteFeed(c *gin.Context) {
	currentUserID := middleware.GetCurrentUserID(c)
	feedIDStr := c.Param("id")
	feedID, err := strconv.ParseUint(feedIDStr, 10, 64)
	if err != nil {
		utils.Error(c, 400, "动态ID无效")
		return
	}

	if err := h.feedService.DeleteFeed(uint(feedID), currentUserID); err != nil {
		utils.Error(c, 400, err.Error())
		return
	}

	utils.SuccessWithMessage(c, "删除成功", nil)
}

// GetFeed 获取动态详情
// GET /api/feeds/:id
func (h *FeedHandler) GetFeed(c *gin.Context) {
	currentUserID := middleware.GetCurrentUserID(c)
	feedIDStr := c.Param("id")
	feedID, err := strconv.ParseUint(feedIDStr, 10, 64)
	if err != nil {
		utils.Error(c, 400, "动态ID无效")
		return
	}

	resp, err := h.feedService.GetFeedByID(uint(feedID), currentUserID)
	if err != nil {
		utils.Error(c, 404, err.Error())
		return
	}

	utils.Success(c, resp)
}

// GetUserFeeds 获取用户动态列表
// GET /api/users/:id/feeds?page=1&page_size=20
func (h *FeedHandler) GetUserFeeds(c *gin.Context) {
	currentUserID := middleware.GetCurrentUserID(c)
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

	feeds, total, err := h.feedService.GetUserFeeds(uint(userID), page, pageSize, currentUserID)
	if err != nil {
		utils.Error(c, 500, "获取动态列表失败")
		return
	}

	utils.SuccessPage(c, feeds, total, page, pageSize)
}

// GetTimeline 获取时间线（游标分页）。
// GET /api/timeline?cursor=0&page_size=20
func (h *FeedHandler) GetTimeline(c *gin.Context) {
	currentUserID := middleware.GetCurrentUserID(c)

	cursor := c.DefaultQuery("cursor", "")
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if pageSize < 1 || pageSize > 50 {
		pageSize = 20
	}

	feeds, nextCursor, hasMore, err := h.feedService.GetTimelineByCursor(currentUserID, cursor, pageSize)
	if err != nil {
		utils.Error(c, 500, "获取时间线失败")
		return
	}

	utils.Success(c, gin.H{
		"list":        feeds,
		"cursor":      cursor,
		"next_cursor": nextCursor,
		"page_size":   pageSize,
		"has_more":    hasMore,
	})
}

// LikeFeed 点赞
// POST /api/feeds/:id/like
func (h *FeedHandler) LikeFeed(c *gin.Context) {
	currentUserID := middleware.GetCurrentUserID(c)
	feedIDStr := c.Param("id")
	feedID, err := strconv.ParseUint(feedIDStr, 10, 64)
	if err != nil {
		utils.Error(c, 400, "动态ID无效")
		return
	}

	if err := h.feedService.LikeFeed(currentUserID, uint(feedID)); err != nil {
		utils.Error(c, 400, err.Error())
		return
	}

	utils.SuccessWithMessage(c, "点赞成功", nil)
}

// UnlikeFeed 取消点赞
// DELETE /api/feeds/:id/like
func (h *FeedHandler) UnlikeFeed(c *gin.Context) {
	currentUserID := middleware.GetCurrentUserID(c)
	feedIDStr := c.Param("id")
	feedID, err := strconv.ParseUint(feedIDStr, 10, 64)
	if err != nil {
		utils.Error(c, 400, "动态ID无效")
		return
	}

	if err := h.feedService.UnlikeFeed(currentUserID, uint(feedID)); err != nil {
		utils.Error(c, 400, err.Error())
		return
	}

	utils.SuccessWithMessage(c, "取消点赞成功", nil)
}

// GetFeedLikers 获取点赞用户列表
// GET /api/feeds/:id/likes?page=1&page_size=20
func (h *FeedHandler) GetFeedLikers(c *gin.Context) {
	feedIDStr := c.Param("id")
	feedID, err := strconv.ParseUint(feedIDStr, 10, 64)
	if err != nil {
		utils.Error(c, 400, "动态ID无效")
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

	likers, total, err := h.feedService.GetFeedLikers(uint(feedID), page, pageSize)
	if err != nil {
		utils.Error(c, 500, "获取点赞列表失败")
		return
	}

	utils.SuccessPage(c, likers, total, page, pageSize)
}

// CommentFeed 评论动态（发根评论或楼内回复）
// POST /api/feeds/:id/comments
// 请求体：{content, parent_id?}；parent_id 省略或 0 = 根评论
func (h *FeedHandler) CommentFeed(c *gin.Context) {
	currentUserID := middleware.GetCurrentUserID(c)
	feedIDStr := c.Param("id")
	feedID, err := strconv.ParseUint(feedIDStr, 10, 64)
	if err != nil {
		utils.Error(c, 400, "动态ID无效")
		return
	}

	var req services.CommentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, 400, "参数错误: "+err.Error())
		return
	}

	comment, err := h.feedService.CommentFeed(currentUserID, uint(feedID), &req)
	if err != nil {
		utils.Error(c, 400, err.Error())
		return
	}

	utils.SuccessWithMessage(c, "评论成功", comment)
}

// DeleteComment 删除评论
// DELETE /api/feeds/:id/comments/:comment_id
func (h *FeedHandler) DeleteComment(c *gin.Context) {
	currentUserID := middleware.GetCurrentUserID(c)

	feedIDStr := c.Param("id")
	feedID, err := strconv.ParseUint(feedIDStr, 10, 64)
	if err != nil {
		utils.Error(c, 400, "动态ID无效")
		return
	}

	commentIDStr := c.Param("comment_id")
	commentID, err := strconv.ParseUint(commentIDStr, 10, 64)
	if err != nil {
		utils.Error(c, 400, "评论ID无效")
		return
	}

	if err := h.feedService.DeleteComment(currentUserID, uint(feedID), uint(commentID)); err != nil {
		utils.Error(c, 400, err.Error())
		return
	}

	utils.SuccessWithMessage(c, "删除评论成功", nil)
}

// GetComments 获取评论列表（语义变更：只返回根评论楼层，内嵌首屏回复）
// GET /api/feeds/:id/comments?page=1&page_size=20
func (h *FeedHandler) GetComments(c *gin.Context) {
	feedIDStr := c.Param("id")
	feedID, err := strconv.ParseUint(feedIDStr, 10, 64)
	if err != nil {
		utils.Error(c, 400, "动态ID无效")
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

	threads, total, err := h.feedService.GetCommentThreads(uint(feedID), page, pageSize)
	if err != nil {
		utils.Error(c, 500, "获取评论列表失败")
		return
	}

	utils.SuccessPage(c, threads, total, page, pageSize)
}

// GetCommentReplies 楼内回复列表（时间正序分页）
// GET /api/feeds/:id/comments/:comment_id/replies?page=1&page_size=10
func (h *FeedHandler) GetCommentReplies(c *gin.Context) {
	feedIDStr := c.Param("id")
	feedID, err := strconv.ParseUint(feedIDStr, 10, 64)
	if err != nil {
		utils.Error(c, 400, "动态ID无效")
		return
	}
	commentIDStr := c.Param("comment_id")
	commentID, err := strconv.ParseUint(commentIDStr, 10, 64)
	if err != nil {
		utils.Error(c, 400, "评论ID无效")
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "10"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 50 {
		pageSize = 10
	}

	replies, total, err := h.feedService.GetFloorReplies(uint(feedID), uint(commentID), page, pageSize)
	if err != nil {
		utils.Error(c, 400, err.Error())
		return
	}

	utils.Success(c, gin.H{
		"list":     replies,
		"total":    total,
		"has_more": int64(page*pageSize) < total,
	})
}

// SearchFeeds 搜索动态
// GET /api/feeds/search?keyword=xxx&page=1&page_size=20
func (h *FeedHandler) SearchFeeds(c *gin.Context) {
	currentUserID := middleware.GetCurrentUserID(c)
	keyword := strings.TrimSpace(c.Query("keyword"))
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

	feeds, total, err := h.feedService.SearchFeeds(keyword, page, pageSize, currentUserID)
	if err != nil {
		utils.Error(c, 500, "搜索动态失败")
		return
	}
	utils.SuccessPage(c, feeds, total, page, pageSize)
}
