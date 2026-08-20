package cache

import (
	"context"
	"feed/config"
	"fmt"
	"log"
	"math/rand"
	"sync/atomic"
	"time"

	"github.com/go-redis/redis/v8"
)

var (
	// RedisClient 为全局 Redis 客户端（进程级单例）。
	RedisClient *redis.Client
	// Ctx 为默认上下文；如需超时控制，调用方可自行构造带超时 ctx。
	Ctx = context.Background()
)

const (
	KeyInbox      = "inbox:%d"     //收件箱
	KeyOutbox     = "outbox:%d"    //发件箱
	KeyFeedDetail = "feed:%d"      //动态详情
	KeyUserInfo   = "user:%d"      //用户信息
	KeyFollowers  = "followers:%d" //粉丝列表
	KeyFollowing  = "following:%d" //关注列表
	KeyIsBigV     = "bigv:%d"      //是否为大V
)

var (
	metricFeedCacheHit    uint64
	metricFeedCacheMiss   uint64
	metricUserCacheHit    uint64
	metricUserCacheMiss   uint64
	metricFeedCacheDelete uint64
	metricUserCacheDelete uint64
	metricFeedDeleteRetry uint64
	metricUserDeleteRetry uint64
)

// InitRedis 初始化 Redis 客户端并执行连通性探测。
func InitRedis() error {
	cfg := config.AppConfig.Redis

	client := redis.NewClient(&redis.Options{
		Addr:     cfg.Addr(),
		Password: cfg.Password,
		DB:       cfg.DB,
		PoolSize: cfg.PoolSize, //连接池大小
	})

	if _, err := client.Ping(Ctx).Result(); err != nil {
		return fmt.Errorf("connect to redis failed: %w", err)
	}

	RedisClient = client
	log.Println("Redis initialized successfully")
	return nil
}

// AddToInbox 将 feedID 添加到收件箱。
func AddToInbox(userID, feedID uint, timestamp float64) error {
	key := fmt.Sprintf(KeyInbox, userID)
	if err := addToSortedSet(key, feedID, timestamp); err != nil {
		return err
	}
	trimSortedSetByMaxSize(key, int64(config.AppConfig.Feed.InboxMaxSize)) //修剪收件箱，保持收件箱大小不超过配置的大小
	return nil
}

// AddToInboxes 批量将 feedID 添加到多个用户收件箱，并按配置修剪大小。
func AddToInboxes(userIDs []uint, feedID uint, timestamp float64) error {
	if len(userIDs) == 0 {
		return nil
	}
	pipe := RedisClient.Pipeline()
	maxSize := int64(config.AppConfig.Feed.InboxMaxSize)
	for _, userID := range userIDs {
		key := fmt.Sprintf(KeyInbox, userID)
		pipe.ZAdd(Ctx, key, &redis.Z{Score: timestamp, Member: feedID})
		if maxSize > 0 {
			pipe.ZRemRangeByRank(Ctx, key, 0, -maxSize-1)
		}
	}
	_, err := pipe.Exec(Ctx)
	return err
}

// AddToOutbox 将 feedID 添加到发件箱。
func AddToOutbox(userID, feedID uint, timestamp float64) error {
	key := fmt.Sprintf(KeyOutbox, userID)
	if err := addToSortedSet(key, feedID, timestamp); err != nil {
		return err
	}
	trimSortedSetByMaxSize(key, int64(config.AppConfig.Feed.OutboxMaxSize)) //修剪发件箱，保持发件箱大小不超过配置的大小
	return nil
}

// GetInbox 按时间倒序获取收件箱 feedID 列表。
func GetInbox(userID uint, offset, limit int64) ([]string, error) {
	key := fmt.Sprintf(KeyInbox, userID)
	return getSortedSetByRangeDesc(key, offset, limit)
}

// GetInboxByScore 按时间倒序获取收件箱 feedID 列表。
func GetInboxByScore(userID uint, minTime, maxTime float64, limit int64) ([]string, error) {
	key := fmt.Sprintf(KeyInbox, userID)
	return RedisClient.ZRevRangeByScore(Ctx, key, &redis.ZRangeBy{
		Min:   fmt.Sprintf("%f", minTime),
		Max:   fmt.Sprintf("%f", maxTime),
		Count: limit,
	}).Result()
}

// GetOutbox 按时间倒序获取发件箱 feedID 列表。
func GetOutbox(userID uint, offset, limit int64) ([]string, error) {
	key := fmt.Sprintf(KeyOutbox, userID)
	return getSortedSetByRangeDesc(key, offset, limit)
}

// GetOutboxByScore 按时间倒序获取发件箱 feedID 列表。
func GetOutboxByScore(userID uint, minTime, maxTime float64, limit int64) ([]string, error) {
	key := fmt.Sprintf(KeyOutbox, userID)
	return RedisClient.ZRevRangeByScore(Ctx, key, &redis.ZRangeBy{
		Min:   fmt.Sprintf("%f", minTime),
		Max:   fmt.Sprintf("%f", maxTime),
		Count: limit,
	}).Result()
}

// RemoveFromInbox 从收件箱中删除 feedID。
func RemoveFromInbox(userID uint, feedID uint) error {
	key := fmt.Sprintf(KeyInbox, userID)
	return RedisClient.ZRem(Ctx, key, feedID).Err()
}

// CacheFeedDetail 缓存 Feed 详情（带随机抖动，防缓存雪崩）。
func CacheFeedDetail(feedID uint, data string) error {
	key := fmt.Sprintf(KeyFeedDetail, feedID)
	ttl := withJitter(24*time.Hour, 10*time.Minute)
	return RedisClient.Set(Ctx, key, data, ttl).Err()
}

// GetFeedDetail 获取Feed详情
func GetFeedDetail(feedID uint) (string, error) {
	key := fmt.Sprintf(KeyFeedDetail, feedID)
	v, err := RedisClient.Get(Ctx, key).Result()
	if err == redis.Nil {
		atomic.AddUint64(&metricFeedCacheMiss, 1)
		return "", err
	}
	if err == nil {
		atomic.AddUint64(&metricFeedCacheHit, 1)
	}
	return v, err
}

// DeleteFeedDetail 主动失效动态详情缓存，用于写操作后的强一致刷新。
func DeleteFeedDetail(feedID uint) error {
	key := fmt.Sprintf(KeyFeedDetail, feedID)
	err := RedisClient.Del(Ctx, key).Err()
	if err == nil {
		atomic.AddUint64(&metricFeedCacheDelete, 1)
	}
	return err
}

// DeleteFeedDetailWithRetry 删除动态详情缓存，失败时按次数异步重试。
func DeleteFeedDetailWithRetry(feedID uint, ttlFallback time.Duration) {
	deleteWithRetry(
		func() error { return DeleteFeedDetail(feedID) },
		func() { atomic.AddUint64(&metricFeedDeleteRetry, 1) },
		ttlFallback,
	)
}

// CacheUserInfo 缓存用户资料（带随机抖动，防缓存雪崩）。
func CacheUserInfo(userID uint, data string) error {
	key := fmt.Sprintf(KeyUserInfo, userID)
	ttl := withJitter(12*time.Hour, 5*time.Minute)
	return RedisClient.Set(Ctx, key, data, ttl).Err()
}

func GetUserInfo(userID uint) (string, error) {
	key := fmt.Sprintf(KeyUserInfo, userID)
	v, err := RedisClient.Get(Ctx, key).Result()
	if err == redis.Nil {
		atomic.AddUint64(&metricUserCacheMiss, 1)
		return "", err
	}
	if err == nil {
		atomic.AddUint64(&metricUserCacheHit, 1)
	}
	return v, err
}

func DeleteUserCache(userID uint) error {
	key := fmt.Sprintf(KeyUserInfo, userID)
	err := RedisClient.Del(Ctx, key).Err()
	if err == nil {
		atomic.AddUint64(&metricUserCacheDelete, 1)
	}
	return err
}

// DeleteUserCacheWithRetry 删除用户详情缓存，失败时按次数异步重试。
func DeleteUserCacheWithRetry(userID uint, ttlFallback time.Duration) {
	deleteWithRetry(
		func() error { return DeleteUserCache(userID) },
		func() { atomic.AddUint64(&metricUserDeleteRetry, 1) },
		ttlFallback,
	)
}

func SetFollowers(userID uint, followerIDs []any) error {
	key := fmt.Sprintf(KeyFollowers, userID)
	if len(followerIDs) == 0 {
		return nil
	}

	pipe := RedisClient.Pipeline()
	pipe.Del(Ctx, key)
	pipe.SAdd(Ctx, key, followerIDs...)
	pipe.Expire(Ctx, key, withJitter(24*time.Hour, 20*time.Minute))
	_, err := pipe.Exec(Ctx)
	return err
}

// SetFollowing 将关注列表批量写入缓存（带随机抖动，防雪崩）。
func SetFollowing(userID uint, followingIDs []any) error {
	key := fmt.Sprintf(KeyFollowing, userID)
	if len(followingIDs) == 0 {
		return nil
	}

	pipe := RedisClient.Pipeline()
	pipe.Del(Ctx, key)
	pipe.SAdd(Ctx, key, followingIDs...)
	pipe.Expire(Ctx, key, withJitter(24*time.Hour, 20*time.Minute))
	_, err := pipe.Exec(Ctx)
	return err
}

func GetFollowers(userID uint) ([]string, error) {
	key := fmt.Sprintf(KeyFollowers, userID)
	return RedisClient.SMembers(Ctx, key).Result()
}

func AddFollower(userID, followerID uint) error {
	key := fmt.Sprintf(KeyFollowers, userID)
	return RedisClient.SAdd(Ctx, key, followerID).Err()
}

func RemoveFollower(userID, followerID uint) error {
	key := fmt.Sprintf(KeyFollowers, userID)
	return RedisClient.SRem(Ctx, key, followerID).Err()
}

func AddFollowing(userID, followedID uint) error {
	key := fmt.Sprintf(KeyFollowing, userID)
	return RedisClient.SAdd(Ctx, key, followedID).Err()
}

func RemoveFollowing(userID, followedID uint) error {
	key := fmt.Sprintf(KeyFollowing, userID)
	return RedisClient.SRem(Ctx, key, followedID).Err()
}

func GetFollowing(userID uint) ([]string, error) {
	key := fmt.Sprintf(KeyFollowing, userID)
	return RedisClient.SMembers(Ctx, key).Result()
}

func IsFollowing(userID, followedID uint) (bool, error) {
	key := fmt.Sprintf(KeyFollowing, userID)
	return RedisClient.SIsMember(Ctx, key, followedID).Result()
}

func SetBigV(userID uint, isBigV bool) error {
	key := fmt.Sprintf(KeyIsBigV, userID)
	if isBigV {
		return RedisClient.Set(Ctx, key, "1", 0).Err()
	}
	return RedisClient.Del(Ctx, key).Err()
}

// IsBigV 判断用户是否为大V（存在 key 即为 true）。
func IsBigV(userID uint) (bool, error) {
	key := fmt.Sprintf(KeyIsBigV, userID)
	exists, err := RedisClient.Exists(Ctx, key).Result()
	if err != nil {
		return false, err
	}
	return exists > 0, nil
}

func addToSortedSet(key string, member uint, score float64) error {
	return RedisClient.ZAdd(Ctx, key, &redis.Z{Score: score, Member: member}).Err()
}

func trimSortedSetByMaxSize(key string, maxSize int64) {
	if maxSize <= 0 {
		return
	}
	_ = RedisClient.ZRemRangeByRank(Ctx, key, 0, -maxSize-1).Err() //修剪有序集合，保持有序集合大小不超过配置的大小
}

// 按时间倒序获取有序集合中的元素
func getSortedSetByRangeDesc(key string, offset, limit int64) ([]string, error) {
	if limit <= 0 {
		return []string{}, nil
	}
	return RedisClient.ZRevRange(Ctx, key, offset, offset+limit-1).Result()
}

func withJitter(baseTTL, maxJitter time.Duration) time.Duration {
	if maxJitter <= 0 {
		return baseTTL
	}
	jitter := time.Duration(rand.Int63n(int64(maxJitter)))
	return baseTTL + jitter
}

// 删除缓存，失败时按次数异步重试。
func deleteWithRetry(deleteFn func() error, onRetry func(), ttlFallback time.Duration) {
	if ttlFallback <= 0 {
		ttlFallback = 24 * time.Hour
	}
	if err := deleteFn(); err != nil {
		onRetry() //添加重试次数计数器
		go func() {
			ticker := time.NewTicker(2 * time.Second)
			defer ticker.Stop()
			expire := time.NewTimer(ttlFallback)
			defer expire.Stop()
			for {
				select {
				case <-ticker.C:
					if retryErr := deleteFn(); retryErr == nil {
						return
					}
				case <-expire.C:
					return
				}
			}
		}()
	}
}

// SnapshotCacheMetrics 返回缓存命中与失效指标快照。
func SnapshotCacheMetrics() map[string]uint64 {
	feedHit := atomic.LoadUint64(&metricFeedCacheHit)
	feedMiss := atomic.LoadUint64(&metricFeedCacheMiss)
	userHit := atomic.LoadUint64(&metricUserCacheHit)
	userMiss := atomic.LoadUint64(&metricUserCacheMiss)

	calcHitRatioBP := func(hit, miss uint64) uint64 {
		total := hit + miss
		if total == 0 {
			return 0
		}
		return hit * 10000 / total
	}

	return map[string]uint64{
		"feed_cache_hit":          feedHit,
		"feed_cache_miss":         feedMiss,
		"feed_cache_hit_ratio_bp": calcHitRatioBP(feedHit, feedMiss),
		"feed_cache_delete":       atomic.LoadUint64(&metricFeedCacheDelete),
		"feed_delete_retry":       atomic.LoadUint64(&metricFeedDeleteRetry),
		"user_cache_hit":          userHit,
		"user_cache_miss":         userMiss,
		"user_cache_hit_ratio_bp": calcHitRatioBP(userHit, userMiss),
		"user_cache_delete":       atomic.LoadUint64(&metricUserCacheDelete),
		"user_delete_retry":       atomic.LoadUint64(&metricUserDeleteRetry),
	}
}
