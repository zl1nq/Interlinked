package services

import (
	"feed/cache"
	"feed/models"
	"feed/repository"
	"strconv"
)

// CleanupDeletedFeed 清理已删除动态在 Redis inbox/outbox 与 timeline 表中的冗余引用。
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
	return repo.DeleteTimelineByFeedID(feedID)
}
