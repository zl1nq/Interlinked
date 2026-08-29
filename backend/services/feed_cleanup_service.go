package services

import (
	"feed/cache"
	"feed/models"
	"feed/repository"
	"strconv"
)

// CleanupDeletedFeed 清理已删除动态的全部关联数据：
// 1) Redis inbox/outbox 中的引用；
// 2) timelines 表的推送记录；
// 3) likes/comments 表的互动数据（物理删除）；
// 4) notifications 表中指向该动态的点赞/评论通知。
// 整体幂等：各步重复执行均为无操作。
func CleanupDeletedFeed(feedID, authorID uint) error {
	repo := repository.NewFeedRepository(models.DB)
	timelines, err := repo.ListTimelinesByFeedID(feedID)
	if err != nil {
		return err
	}

	pipe := cache.RedisClient.Pipeline()
	feedKey := strconv.FormatUint(uint64(feedID), 10)
	if authorID > 0 {
		pipe.ZRem(cache.Ctx, "outbox:"+strconv.FormatUint(uint64(authorID), 10), feedKey)
	}
	for _, tl := range timelines {
		pipe.ZRem(cache.Ctx, "inbox:"+strconv.FormatUint(uint64(tl.UserID), 10), feedKey)
	}
	if _, err := pipe.Exec(cache.Ctx); err != nil {
		return err
	}
	if err := repo.DeleteTimelineByFeedID(feedID); err != nil {
		return err
	}
	if err := repo.DeleteLikesByFeedID(feedID); err != nil {
		return err
	}
	if err := repo.DeleteCommentsByFeedID(feedID); err != nil {
		return err
	}
	return NewNotificationService().CleanupFeedNotifications(feedID)
}
