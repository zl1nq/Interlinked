package services

import (
	"encoding/json"
	"errors"
	"feed/cache"
	"feed/models"
	"feed/repository"
	"log"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"
)

// FeedService 负责动态域的业务编排：发布、时间线聚合、互动（赞/评/转）等。
// 说明：
// 1) service 层只做业务规则与流程控制；
// 2) 数据访问下沉到 repository；
// 3) 分发相关能力委托给 mq/cache。
type FeedService struct {
	userService         *UserService
	feedRepo            repository.FeedRepository
	outboxRepo          repository.OutboxRepository
	userRepo            repository.UserRepository
	followRepo          repository.FollowRepository
	notificationService *NotificationService
}

// NewFeedService 构建 FeedService。
func NewFeedService() *FeedService {
	return &FeedService{
		userService:         &UserService{},
		feedRepo:            repository.NewFeedRepository(models.DB),
		outboxRepo:          repository.NewOutboxRepository(models.DB),
		userRepo:            repository.NewUserRepository(models.DB),
		followRepo:          repository.NewFollowRepository(models.DB),
		notificationService: NewNotificationService(),
	}
}

// CreateFeedRequest 发布Feed请求
type CreateFeedRequest struct {
	Content string `json:"content"`
	Images  string `json:"images"`
	Videos  string `json:"videos"`
}

// UpdateFeedRequest 编辑Feed请求
type UpdateFeedRequest struct {
	Content string `json:"content"`
	Images  string `json:"images"`
	Videos  string `json:"videos"`
}

// RepostFeedRequest 转发Feed请求
type RepostFeedRequest struct {
	Content    string `json:"content"`
	OriginalID uint   `json:"original_id" binding:"required"`
}

// PublishFeed 发布动态：
// 1) 先做参数约束（文案/图片/视频至少其一）；
// 2) 同一事务写入 feed 主表和 outbox 事件；
// 3) 由后台 worker 可靠投递 MQ。
func (s *FeedService) PublishFeed(userID uint, req *CreateFeedRequest) (*models.Feed, error) {
	if req.Content == "" && req.Images == "" && req.Videos == "" {
		return nil, errors.New("请输入文案、上传图片或视频")
	}
	feed := &models.Feed{UserID: userID, Content: req.Content, Images: req.Images, Videos: req.Videos, FeedType: models.FeedTypeOriginal}
	tx := s.feedRepo.BeginTx()
	if err := s.feedRepo.CreateInTx(tx, feed); err != nil {
		tx.Rollback()
		return nil, errors.New("发布失败")
	}
	if err := s.createFeedPublishedOutbox(tx, feed.ID, userID); err != nil {
		tx.Rollback()
		return nil, errors.New("发布失败")
	}
	if err := tx.Commit().Error; err != nil {
		return nil, errors.New("发布失败")
	}
	cache.AddFeedID(feed.ID) //添加动态ID到布隆过滤器
	return feed, nil
}

func (s *FeedService) createFeedPublishedOutbox(tx *gorm.DB, feedID, userID uint) error {
	return s.createOutboxEvent(tx, models.OutboxEventTypeFeedPublished, feedID, map[string]any{"feed_id": feedID, "author_id": userID})
}

func (s *FeedService) createFeedDeletedOutbox(tx *gorm.DB, feedID, userID uint) error {
	return s.createOutboxEvent(tx, models.OutboxEventTypeFeedDeleted, feedID, map[string]any{"feed_id": feedID, "author_id": userID})
}

func (s *FeedService) createOutboxEvent(tx *gorm.DB, eventType string, aggregateID uint, payloadData map[string]any) error {
	payload, err := json.Marshal(payloadData)
	if err != nil {
		return err
	}
	return s.outboxRepo.CreateInTx(tx, &models.OutboxEvent{
		EventType:   eventType,
		AggregateID: aggregateID,
		Payload:     string(payload),
		Status:      models.OutboxEventStatusPending,
		NextRetryAt: time.Now(),
	})
}

// 更新动态
func (s *FeedService) UpdateFeed(feedID, userID uint, req *UpdateFeedRequest) (*models.Feed, error) {
	//获取动态
	feed, err := s.feedRepo.GetByIDAndUserID(feedID, userID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("动态不存在或无权限编辑")
		}
		return nil, err
	}

	updates := make(map[string]any)
	if req.Content != "" {
		updates["content"] = req.Content //更新内容
	}
	if feed.FeedType == models.FeedTypeOriginal {
		if req.Images != "" {
			updates["images"] = req.Images //更新图片
		}
		if req.Videos != "" {
			updates["videos"] = req.Videos //更新视频
		}
	}
	if len(updates) == 0 {
		return nil, errors.New("请至少填写一个要更新的字段") //返回错误
	}
	if err := s.feedRepo.UpdateByID(feedID, updates); err != nil { //更新动态
		return nil, errors.New("编辑失败") //返回错误
	}
	cache.DeleteFeedDetailWithRetry(feedID, 24*time.Hour) //删除动态详情缓存 带异步重试
	return s.feedRepo.GetByID(feedID)                     //获取动态
}

// 转发动态：转发原始动态，并增加转发数
func (s *FeedService) RepostFeed(userID uint, req *RepostFeedRequest) (*models.Feed, error) {
	originalFeed, err := s.feedRepo.GetByID(req.OriginalID) //获取原始动态
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("原始动态不存在")
		}
		return nil, err
	}
	//获取原始动态ID 如果原始动态是转发动态，则获取原始动态ID
	originalID := req.OriginalID
	if originalFeed.FeedType == models.FeedTypeRepost && originalFeed.OriginalID != nil {
		originalID = *originalFeed.OriginalID
	}
	//创建转发动态
	feed := &models.Feed{UserID: userID, Content: req.Content, FeedType: models.FeedTypeRepost, OriginalID: &originalID}
	tx := s.feedRepo.BeginTx()                    //开启事务
	if err := tx.Create(feed).Error; err != nil { //创建转发动态
		tx.Rollback()
		return nil, errors.New("转发失败")
	}
	if err := s.feedRepo.IncreaseShareCount(tx, originalID); err != nil { //增加转发数
		tx.Rollback()
		return nil, err
	}
	if err := s.createFeedPublishedOutbox(tx, feed.ID, userID); err != nil {
		tx.Rollback()
		return nil, errors.New("转发失败")
	}
	if err := tx.Commit().Error; err != nil { //提交事务
		return nil, errors.New("转发失败")
	}
	cache.AddFeedID(feed.ID) //添加动态ID到布隆过滤器
	return feed, nil
}

// 删除动态：删除动态，并删除缓存
func (s *FeedService) DeleteFeed(feedID, userID uint) error {
	feed, err := s.feedRepo.GetByIDAndUserID(feedID, userID) //获取动态 并判断是否有权限删除
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("动态不存在或无权限删除")
		}
		return err
	}
	tx := s.feedRepo.BeginTx()
	if err := tx.Delete(feed).Error; err != nil {
		tx.Rollback()
		return errors.New("删除失败")
	}
	if err := s.createFeedDeletedOutbox(tx, feedID, userID); err != nil {
		tx.Rollback()
		return errors.New("删除失败")
	}
	if err := tx.Commit().Error; err != nil {
		return errors.New("删除失败")
	}
	cache.DeleteFeedDetailWithRetry(feedID, 24*time.Hour) //删除动态详情缓存 带异步重试

	return nil //返回成功
}

// GetFeedByID 获取动态详情：优先读缓存，缓存未命中时 setnx回源请求。//获取动态详情 并判断是否有权限查看
func (s *FeedService) GetFeedByID(feedID, currentUserID uint) (*models.FeedResponse, error) {
	if !cache.MightFeedExist(feedID) { //判断动态是否存在
		return nil, errors.New("动态不存在")
	}
	//获取动态详情
	if raw, err := cache.GetFeedDetail(feedID); err == nil && raw != "" {
		var resp models.FeedResponse
		if unmarshalErr := json.Unmarshal([]byte(raw), &resp); unmarshalErr == nil {
			if currentUserID > 0 { //判断是否点赞
				if like, likeErr := s.feedRepo.GetLike(currentUserID, feedID); likeErr == nil && like != nil {
					resp.IsLiked = true
				}
			}
			return &resp, nil
		}
	}
	//setnx 获取锁 回源请求
	lockKey := "lock:feed_detail:" + strconv.FormatUint(uint64(feedID), 10) //获取锁
	lockToken := strconv.FormatInt(time.Now().UnixMilli(), 10)
	locked, _ := cache.AcquireLock(lockKey, lockToken, 3*time.Second) //获取锁
	if !locked {
		for range 8 {
			time.Sleep(40 * time.Millisecond)                                     //等待锁
			if raw, err := cache.GetFeedDetail(feedID); err == nil && raw != "" { //获取动态详情
				var resp models.FeedResponse
				if unmarshalErr := json.Unmarshal([]byte(raw), &resp); unmarshalErr == nil { //解析动态详情
					if currentUserID > 0 {
						if like, likeErr := s.feedRepo.GetLike(currentUserID, feedID); likeErr == nil && like != nil { //判断是否点赞
							resp.IsLiked = true
						}
					}
					return &resp, nil //返回动态详情
				}
			}
		}
	}
	if locked {
		defer func() { _ = cache.ReleaseLock(lockKey, lockToken) }() //释放锁
	}

	feed, dbErr := s.feedRepo.GetByID(feedID) //获取动态
	if dbErr != nil {
		if errors.Is(dbErr, gorm.ErrRecordNotFound) { //判断动态是否存在
			return nil, errors.New("动态不存在")
		}
		return nil, dbErr
	}
	resp := s.buildFeedResponse(feed, currentUserID) //构建动态详情
	cachePayload := *resp
	cachePayload.IsLiked = false                            //设置是否点赞
	if b, mErr := json.Marshal(cachePayload); mErr == nil { //序列化动态详情
		_ = cache.CacheFeedDetail(feedID, string(b)) //缓存动态详情
	}
	return resp, nil //返回动态详情
}

// 获取用户动态：获取用户动态，并构建动态详情
func (s *FeedService) GetUserFeeds(userID uint, page, pageSize int, currentUserID uint) ([]models.FeedResponse, int64, error) {
	feeds, total, err := s.feedRepo.ListByUserID(userID, page, pageSize) //获取用户动态
	if err != nil {
		return nil, 0, err
	}
	responses := s.buildFeedResponses(feeds, currentUserID) //构建动态详情
	return responses, total, nil                            //返回动态详情
}

// GetTimeline 获取时间线（真正游标分页）。//获取时间线 并判断是否有权限查看
// cursor 语义："created_at_unix_Milli|feed_id"，首次传空字符串。
// 返回 nextCursor 继续向更旧内容翻页。
func (s *FeedService) GetTimelineByCursor(userID uint, cursor string, pageSize int) ([]models.FeedResponse, string, bool, error) {
	if pageSize <= 0 { //判断页码是否小于等于0
		pageSize = 20
	}
	if pageSize > 50 { //判断页码是否大于50
		pageSize = 50
	}
	//解析游标
	cursorTime, cursorID := parseTimelineCursor(cursor) //解析游标
	prefetchSize := pageSize + 1
	candidateIDs := make([]uint, 0, prefetchSize*8) //候选动态ID
	//拉取自己的动态
	ownFeeds, _ := s.feedRepo.ListRecentByUserID(userID, prefetchSize*4) //拉取自己的动态
	ownIDs := s.filterTimelineCandidates(ownFeeds, cursorTime, cursorID)
	candidateIDs = append(candidateIDs, ownIDs...) //把自己的动态ID添加到候选动态ID
	//拉取收件箱
	if inboxIDs, err := cache.GetInboxByScore(userID, -1, float64(cursorTime.UnixMilli()), int64(prefetchSize*6)); err == nil { //拉取收件箱
		for _, idStr := range inboxIDs {
			id, _ := strconv.ParseUint(idStr, 10, 64)
			if id > 0 {
				candidateIDs = append(candidateIDs, uint(id)) //把收件箱动态ID添加到候选动态ID
			}
		}
	} else {
		log.Printf("Get inbox from redis failed: %v", err) //获取收件箱失败
	}
	//拉取大V的发件箱
	bigVIDs := s.pullBigVFeeds(userID, int64(prefetchSize*2), cursorTime)
	candidateIDs = append(candidateIDs, bigVIDs...) //把大V的发件箱动态ID添加到候选动态ID
	candidateIDs = s.uniqueUint(candidateIDs)       //去重
	if len(candidateIDs) == 0 {
		return []models.FeedResponse{}, cursor, false, nil
	}
	//拉取动态
	feeds, err := s.feedRepo.ListByIDsBeforeCursor(candidateIDs, cursorTime, cursorID)
	if err != nil {
		return nil, cursor, false, err
	}
	//分页
	pageFeeds, hasMore := pickTimelinePage(feeds, pageSize)
	if len(pageFeeds) == 0 {
		return []models.FeedResponse{}, cursor, false, nil
	}

	responses := s.buildFeedResponses(pageFeeds, userID)
	last := pageFeeds[len(pageFeeds)-1]
	//构建游标
	nextCursor := buildTimelineCursor(last.CreatedAt, last.ID)
	if len(feeds) > len(pageFeeds) {
		hasMore = true
	}
	//判断是否还有更多
	if !hasMore && len(pageFeeds) == pageSize {
		hasMore = s.hasMoreTimelineAfter(userID, last.CreatedAt, last.ID)
	}
	return responses, nextCursor, hasMore, nil
}

// 判断是否还有更多
func (s *FeedService) hasMoreTimelineAfter(userID uint, lastTime time.Time, lastID uint) bool {
	ownFeeds, _ := s.feedRepo.ListRecentByUserID(userID, 100)        //拉取自己的动态
	ownIDs := s.filterTimelineCandidates(ownFeeds, lastTime, lastID) //过滤动态
	if len(ownIDs) > 0 {
		return true
	}
	//拉取收件箱
	inboxIDs, err := cache.GetInboxByScore(userID, -1, float64(lastTime.UnixMilli()), 100)
	if err != nil || len(inboxIDs) == 0 {
		return false
	}

	ids := make([]uint, 0, len(inboxIDs))
	for _, idStr := range inboxIDs {
		id, _ := strconv.ParseUint(idStr, 10, 64)
		if id > 0 {
			ids = append(ids, uint(id))
		}
	}
	//拉取动态
	feeds, err := s.feedRepo.ListByIDsNewerThanCursor(ids, lastTime, lastID)
	if err != nil {
		return false
	}
	return len(feeds) > 0
}

// pullBigVFeeds 拉取当前用户所关注大V的发件箱内容。
// 若 Redis 不可用则回源 DB 最近动态，保证可用性优先。
// pullBigVFeeds 拉取当前用户关注的大V发件箱动态。
// 若 Redis outbox 缺失，则回源数据库兜底。
func (s *FeedService) pullBigVFeeds(userID uint, limit int64, cursorTime time.Time) []uint {
	//拉取当前用户关注的大V
	follows, _ := s.followRepo.ListFollowingAll(userID)
	seen := make(map[uint]bool)                             //去重
	pullFeedIDs := make([]uint, 0, len(follows)*int(limit)) //拉取动态ID
	cursorMilli := float64(cursorTime.UnixMilli())

	for _, follow := range follows {
		isBigV, err := cache.IsBigV(follow.FollowedID) //判断是否大V
		if err != nil {
			if user, dbErr := s.userRepo.GetByID(follow.FollowedID); dbErr == nil { //获取用户信息
				isBigV = user.IsBigV
			}
		}
		if !isBigV { //判断是否大V
			continue
		}

		appendFeedID := func(id uint) { //添加动态ID
			if id == 0 || seen[id] { //判断动态ID是否为0 或者 是否已经存在
				return
			}
			seen[id] = true //设置动态ID为已存在
			pullFeedIDs = append(pullFeedIDs, id)
		}

		outboxIDs, err := cache.GetOutboxByScore(follow.FollowedID, -1, cursorMilli, limit) //拉取发件箱动态ID
		if err != nil || len(outboxIDs) == 0 {
			feeds, _ := s.feedRepo.ListRecentByUserID(follow.FollowedID, int(limit)) //拉取动态
			for _, feed := range feeds {
				if feed.CreatedAt.After(cursorTime) || feed.CreatedAt.Equal(cursorTime) { //判断动态创建时间是否大于游标时间 或者 动态创建时间等于游标时间
					continue
				}
				appendFeedID(feed.ID) //添加动态ID
			}
			continue
		}
		for _, idStr := range outboxIDs { //拉取发件箱动态ID
			id, _ := strconv.ParseUint(idStr, 10, 64)
			appendFeedID(uint(id))
		}
	}
	return pullFeedIDs
}

// 合并并去重
func (s *FeedService) mergeAndDedup(idGroups ...[]uint) []uint {
	seen := make(map[uint]bool)
	var result []uint
	for _, ids := range idGroups {
		for _, id := range ids {
			if !seen[id] {
				seen[id] = true
				result = append(result, id)
			}
		}
	}
	return result
}

// 去重
func (s *FeedService) uniqueUint(values []uint) []uint {
	seen := make(map[uint]bool)
	result := make([]uint, 0, len(values))
	for _, v := range values {
		if !seen[v] {
			seen[v] = true
			result = append(result, v)
		}
	}
	return result
}

// 构建动态详情
func (s *FeedService) buildFeedResponse(feed *models.Feed, currentUserID uint) *models.FeedResponse {
	//获取用户信息
	author, _ := s.userRepo.GetByID(feed.UserID) //获取用户信息
	resp := &models.FeedResponse{ID: feed.ID, UserID: feed.UserID, Content: feed.Content, Images: feed.Images, Videos: feed.Videos, FeedType: feed.FeedType, OriginalID: feed.OriginalID, LikeCount: feed.LikeCount, CommentCount: feed.CommentCount, ShareCount: feed.ShareCount, CreatedAt: feed.CreatedAt, UpdatedAt: feed.UpdatedAt}
	if author != nil { //判断用户信息是否为空
		resp.Author = author.ToResponse() //构建用户信息
	}
	if feed.FeedType == models.FeedTypeRepost && feed.OriginalID != nil { //判断动态类型是否为转发 并且 原始动态ID是否为空
		if originalFeed, err := s.feedRepo.GetByID(*feed.OriginalID); err == nil { //获取原始动态
			origResp := s.buildOriginalFeedResponse(originalFeed) //构建原始动态信息
			resp.OriginalFeed = origResp
		}
	}
	if currentUserID > 0 { //判断当前用户ID是否大于0
		if like, err := s.feedRepo.GetLike(currentUserID, feed.ID); err == nil && like != nil { //判断是否点赞
			resp.IsLiked = true //设置是否点赞
		}
	}
	return resp //返回动态详情
}

// 构建原始动态详情
func (s *FeedService) buildOriginalFeedResponse(feed *models.Feed) *models.FeedResponse {
	//获取用户信息
	author, _ := s.userRepo.GetByID(feed.UserID) //获取用户信息
	resp := &models.FeedResponse{ID: feed.ID, UserID: feed.UserID, Content: feed.Content, Images: feed.Images, Videos: feed.Videos, FeedType: feed.FeedType, LikeCount: feed.LikeCount, CommentCount: feed.CommentCount, ShareCount: feed.ShareCount, CreatedAt: feed.CreatedAt, UpdatedAt: feed.UpdatedAt}
	if author != nil { //判断用户信息是否为空
		resp.Author = author.ToResponse() //构建用户信息
	}
	return resp //返回原始动态详情
}

// buildFeedResponses 批量构建动态响应，减少 N+1 查询：
// 1) 批量加载作者
// 2) 批量加载当前用户点赞状态
// 3) 批量加载转发原文与原作者
func (s *FeedService) buildFeedResponses(feeds []models.Feed, currentUserID uint) []models.FeedResponse {
	//判断动态是否为空
	if len(feeds) == 0 {
		return []models.FeedResponse{}
	}

	userIDs := make([]uint, 0, len(feeds)) //用户ID
	for _, feed := range feeds {
		userIDs = append(userIDs, feed.UserID)
	}

	feedIDs := make([]uint, 0, len(feeds)) //动态ID
	for _, feed := range feeds {
		feedIDs = append(feedIDs, feed.ID)
	}

	originalIDs := make([]uint, 0) //原始动态ID
	for _, feed := range feeds {
		if feed.FeedType == models.FeedTypeRepost && feed.OriginalID != nil {
			originalIDs = append(originalIDs, *feed.OriginalID)
		}
	}

	originals := make([]models.Feed, 0)
	if len(originalIDs) > 0 { //判断原始动态ID是否大于0
		originals, _ = s.feedRepo.ListByIDs(s.uniqueUint(originalIDs))
		for _, orig := range originals {
			userIDs = append(userIDs, orig.UserID) //添加用户ID
		}
	}

	userIDs = s.uniqueUint(userIDs)
	users, _ := s.userRepo.ListByIDs(userIDs) //获取用户信息
	userMap := map[uint]models.User{}
	for _, u := range users {
		userMap[u.ID] = u
	}

	likedMap := map[uint]bool{} //点赞状态
	if currentUserID > 0 {
		likes, _ := s.feedRepo.ListLikesByUserAndFeedIDs(currentUserID, feedIDs)
		for _, like := range likes {
			likedMap[like.FeedID] = true //设置点赞状态
		}
	}

	originalMap := map[uint]models.Feed{} //原始动态
	for _, orig := range originals {
		originalMap[orig.ID] = orig //设置原始动态
	}

	responses := make([]models.FeedResponse, 0, len(feeds)) //动态详情
	for _, feed := range feeds {
		resp := models.FeedResponse{ID: feed.ID, UserID: feed.UserID, Content: feed.Content, Images: feed.Images, Videos: feed.Videos, FeedType: feed.FeedType, OriginalID: feed.OriginalID, LikeCount: feed.LikeCount, CommentCount: feed.CommentCount, ShareCount: feed.ShareCount, CreatedAt: feed.CreatedAt, UpdatedAt: feed.UpdatedAt, IsLiked: likedMap[feed.ID]}
		if author, ok := userMap[feed.UserID]; ok {
			resp.Author = author.ToResponse() //构建用户信息
		}
		if feed.FeedType == models.FeedTypeRepost && feed.OriginalID != nil {
			if orig, ok := originalMap[*feed.OriginalID]; ok {
				origResp := &models.FeedResponse{ID: orig.ID, UserID: orig.UserID, Content: orig.Content, Images: orig.Images, Videos: orig.Videos, FeedType: orig.FeedType, LikeCount: orig.LikeCount, CommentCount: orig.CommentCount, ShareCount: orig.ShareCount, CreatedAt: orig.CreatedAt} //构建原始动态信息
				if oa, ok2 := userMap[orig.UserID]; ok2 {
					origResp.Author = oa.ToResponse() //构建用户信息
				}
				resp.OriginalFeed = origResp
			}
		}
		responses = append(responses, resp) //添加动态详情
	}
	return responses //返回动态详情
}

// LikeFeed 点赞动态，并在成功后失效该动态详情缓存，避免计数陈旧。
func (s *FeedService) LikeFeed(userID, feedID uint) error {
	if _, err := s.feedRepo.GetByID(feedID); err != nil { //获取动态
		return errors.New("动态不存在")
	}
	if like, err := s.feedRepo.GetLike(userID, feedID); err == nil && like != nil { //获取点赞
		return errors.New("已点赞")
	}
	tx := s.feedRepo.BeginTx() //开始事务
	like := &models.Like{UserID: userID, FeedID: feedID, CreatedAt: time.Now()}
	if err := s.feedRepo.CreateLike(tx, like); err != nil {
		tx.Rollback() //回滚
		return errors.New("点赞失败")
	}
	if err := s.feedRepo.IncreaseLikeCount(tx, feedID); err != nil { //增加点赞数
		tx.Rollback()
		return err
	}
	if err := tx.Commit().Error; err != nil { //提交事务
		return errors.New("点赞失败")
	}
	cache.DeleteFeedDetailWithRetry(feedID, 24*time.Hour) //删除动态详情缓存
	if feed, err := s.feedRepo.GetByID(feedID); err == nil {
		s.notificationService.CreateLikeNotification(userID, feed.UserID, feedID) //创建点赞通知
	}
	return nil
}

// UnlikeFeed 取消点赞，并失效动态详情缓存，保证读到最新计数。
func (s *FeedService) UnlikeFeed(userID, feedID uint) error {
	like, err := s.feedRepo.GetLike(userID, feedID) //获取点赞
	if err != nil || like == nil {
		return errors.New("未点赞")
	}
	tx := s.feedRepo.BeginTx() //开始事务
	if err := s.feedRepo.DeleteLike(tx, like); err != nil {
		tx.Rollback() //回滚
		return errors.New("取消点赞失败")
	}
	if err := s.feedRepo.DecreaseLikeCount(tx, feedID); err != nil { //减少点赞数
		tx.Rollback()
		return err
	}
	if err := tx.Commit().Error; err != nil { //提交事务
		return errors.New("取消点赞失败")
	}
	cache.DeleteFeedDetailWithRetry(feedID, 24*time.Hour) //删除动态详情缓存
	return nil
}

// 获取动态点赞者
func (s *FeedService) GetFeedLikers(feedID uint, page, pageSize int) ([]models.LikeResponse, int64, error) {
	//获取动态点赞者
	likes, total, err := s.feedRepo.ListLikesByFeedID(feedID, page, pageSize) //获取动态点赞者
	if err != nil {
		return nil, 0, err
	}
	if len(likes) == 0 {
		return []models.LikeResponse{}, total, nil //返回点赞者
	}
	userIDs := make([]uint, 0, len(likes)) //用户ID
	for _, like := range likes {
		userIDs = append(userIDs, like.UserID)
	}
	users, err := s.userRepo.ListByIDs(userIDs) //获取用户信息
	if err != nil {
		return nil, 0, err
	}
	userMap := make(map[uint]models.User) //用户信息
	for _, u := range users {
		userMap[u.ID] = u //设置用户信息
	}
	result := make([]models.LikeResponse, 0, len(likes)) //点赞者
	for _, like := range likes {
		if u, ok := userMap[like.UserID]; ok {
			result = append(result, models.LikeResponse{
				ID:       like.ID,
				UserID:   like.UserID,
				FeedID:   like.FeedID,
				Username: u.Username,
				Nickname: u.Nickname,
			})
		}
	}
	return result, total, nil //返回点赞者
}

// CommentFeed 发布评论，并失效动态详情缓存，保证评论数及时刷新。
func (s *FeedService) CommentFeed(userID, feedID uint, content string) (*models.Comment, error) {
	if _, err := s.feedRepo.GetByID(feedID); err != nil { //获取动态
		return nil, errors.New("动态不存在")
	}
	tx := s.feedRepo.BeginTx() //开始事务
	comment := &models.Comment{UserID: userID, FeedID: feedID, Content: content}
	if err := s.feedRepo.CreateComment(tx, comment); err != nil {
		tx.Rollback() //回滚
		return nil, errors.New("评论失败")
	}
	if err := s.feedRepo.IncreaseCommentCount(tx, feedID); err != nil { //增加评论数
		tx.Rollback()
		return nil, err
	}
	if err := tx.Commit().Error; err != nil { //提交事务
		return nil, errors.New("评论失败")
	}
	cache.DeleteFeedDetailWithRetry(feedID, 24*time.Hour) //删除动态详情缓存
	if feed, err := s.feedRepo.GetByID(feedID); err == nil {
		s.notificationService.CreateCommentNotification(userID, feed.UserID, feedID, content) //创建评论通知
	}
	return comment, nil //返回评论
}

// DeleteComment 删除评论，并失效动态详情缓存，避免评论数不一致。
func (s *FeedService) DeleteComment(currentUserID, feedID, commentID uint) error {
	//获取评论
	comment, err := s.feedRepo.GetCommentByIDAndFeedID(commentID, feedID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("评论不存在")
		}
		return err
	}
	if comment.UserID != currentUserID { //判断评论用户ID是否等于当前用户ID
		return errors.New("无权限删除该评论")
	}
	tx := s.feedRepo.BeginTx() //开始事务
	if err := s.feedRepo.DeleteComment(tx, comment); err != nil {
		tx.Rollback() //回滚
		return errors.New("删除评论失败")
	}
	if err := s.feedRepo.DecreaseCommentCount(tx, feedID); err != nil { //减少评论数
		tx.Rollback()
		return err
	}
	if err := tx.Commit().Error; err != nil { //提交事务
		return errors.New("删除评论失败")
	}
	cache.DeleteFeedDetailWithRetry(feedID, 24*time.Hour) //删除动态详情缓存
	return nil
}

// 获取评论
func (s *FeedService) GetComments(feedID uint, page, pageSize int) ([]models.CommentResponse, int64, error) {
	//获取评论
	comments, total, err := s.feedRepo.ListCommentsByFeedID(feedID, page, pageSize)
	if err != nil {
		return nil, 0, err
	}
	userIDs := make([]uint, 0, len(comments)) //用户ID
	for _, c := range comments {
		userIDs = append(userIDs, c.UserID)
	}
	users, _ := s.userRepo.ListByIDs(userIDs) //获取用户信息
	userMap := map[uint]models.User{}
	for _, u := range users {
		userMap[u.ID] = u //设置用户信息
	}
	result := make([]models.CommentResponse, 0, len(comments)) //评论详情
	for _, comment := range comments {
		if u, ok := userMap[comment.UserID]; ok {
			result = append(result, models.CommentResponse{
				ID:       comment.ID,
				UserID:   comment.UserID,
				FeedID:   comment.FeedID,
				Content:  comment.Content,
				Username: u.Username,
				Nickname: u.Nickname,
			})
		}
	}
	return result, total, nil
}

// SearchFeeds 搜索动态内容，用于全局搜索“动态”tab。
func (s *FeedService) SearchFeeds(keyword string, page, pageSize int, currentUserID uint) ([]models.FeedResponse, int64, error) {
	//搜索动态
	feeds, total, err := s.feedRepo.SearchByKeyword(keyword, page, pageSize) //搜索动态
	if err != nil {
		return nil, 0, err
	}
	return s.buildFeedResponses(feeds, currentUserID), total, nil //返回动态
}

// 解析时间线游标
func parseTimelineCursor(cursor string) (time.Time, uint) {
	maxTimelineTime := time.Date(9999, 12, 30, 23, 59, 59, 999999999, time.UTC) //最大时间线时间
	if strings.TrimSpace(cursor) == "" {
		return maxTimelineTime, ^uint(0) //返回最大时间线时间
	}
	parts := strings.Split(cursor, "|")
	if len(parts) != 2 {
		return maxTimelineTime, ^uint(0) //返回最大时间线时间
	}
	milli, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return maxTimelineTime, ^uint(0) //返回最大时间线时间
	}
	id, err := strconv.ParseUint(parts[1], 10, 64)
	if err != nil {
		return maxTimelineTime, ^uint(0) //返回最大时间线时间
	}
	return time.UnixMilli(milli), uint(id) //返回时间线时间
}

// 构建时间线游标
func buildTimelineCursor(createdAt time.Time, id uint) string {
	//如果创建时间等于0 则设置为最大时间线时间
	if createdAt.IsZero() {
		createdAt = time.Date(9999, 12, 30, 23, 59, 59, 999999999, time.UTC)
	}
	return strconv.FormatInt(createdAt.UnixMilli(), 10) + "|" + strconv.FormatUint(uint64(id), 10) //返回时间线游标
}

// 过滤时间线候选动态
func (s *FeedService) filterTimelineCandidates(feeds []models.Feed, cursorTime time.Time, cursorID uint) []uint {
	ids := make([]uint, 0, len(feeds)) //动态ID
	for _, feed := range feeds {
		//如果动态创建时间大于游标时间 或者 动态创建时间等于游标时间且动态ID大于等于游标ID 则跳过
		if feed.CreatedAt.After(cursorTime) || (feed.CreatedAt.Equal(cursorTime) && feed.ID >= cursorID) {
			continue
		}
		ids = append(ids, feed.ID)
	}
	return ids
}

// 选择时间线页面
func pickTimelinePage(feeds []models.Feed, pageSize int) ([]models.Feed, bool) {
	//如果动态为空 返回空数组和false
	if len(feeds) == 0 {
		return []models.Feed{}, false
	}
	//如果动态数量小于等于分页大小 返回动态和false
	if len(feeds) <= pageSize {
		return feeds, false
	}
	return feeds[:pageSize], true
}
