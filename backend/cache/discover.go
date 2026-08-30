// discover.go 发现页热门排行的 Redis 缓存。
// 缓存内容为排行 ID 列表（前 N 名，按排行顺序）+ 全站 total：
// - 不缓存拼装后的响应，因为 is_liked / is_followed 是登录用户视角字段；
// - 不做写时失效，榜单对实时性不敏感，依赖短 TTL 自然过期。
// Key 结构：discover:popular:{users|feeds}:v1（版本号便于口径变更时整体作废）。
package cache

import (
	"encoding/json"
	"time"
)

const (
	DiscoverKeyUsers = "discover:popular:users:v1"
	DiscoverKeyFeeds = "discover:popular:feeds:v1"

	discoverRankingBaseTTL  = 60 * time.Second
	discoverRankingJitter   = 10 * time.Second
	discoverRankingMaxItems = 200
)

// DiscoverRanking 排行榜缓存结构。
type DiscoverRanking struct {
	Total int64  `json:"total"` // 全站符合条件总数（用户榜不含排除语义，排除在读路径处理）
	IDs   []uint `json:"ids"`   // 排行 ID，按榜单顺序
}

// GetDiscoverRanking 读取排行缓存；未命中或损坏返回 error。
func GetDiscoverRanking(key string) (*DiscoverRanking, error) {
	raw, err := RedisClient.Get(Ctx, key).Result()
	if err != nil {
		return nil, err
	}
	var r DiscoverRanking
	if err := json.Unmarshal([]byte(raw), &r); err != nil {
		return nil, err
	}
	return &r, nil
}

// SetDiscoverRanking 写入排行缓存，TTL 60s + 随机抖动防雪崩。
func SetDiscoverRanking(key string, total int64, ids []uint) {
	b, err := json.Marshal(DiscoverRanking{Total: total, IDs: ids})
	if err != nil {
		return
	}
	_ = RedisClient.Set(Ctx, key, b, withJitter(discoverRankingBaseTTL, discoverRankingJitter)).Err()
}
