// discover_service.go 发现页热门推荐业务编排。
// 排行口径（与 docs/发现页热门推荐API.md 定版一致）：
// - 热门用户：follower_count 全站历史累计降序，排除当前登录用户；
// - 热门动态：like_count 全站历史累计降序，原创与转发同榜；
// - is_followed / is_liked 为当前登录用户视角，供前端展示交互态。
// 缓存策略：排行前 200 名的 ID 列表 + total 缓存 Redis（60s TTL + 抖动，不主动失效），
// 分页在内存切片，详情实时回查并按缓存顺序重排；超出缓存范围或 Redis 不可用时回落实时查库。
package services

import (
	"feed/cache"
	"feed/models"
	"feed/repository"
)

// discoverRankingSize 榜单缓存长度：覆盖到第 4 页（50 条/页），更深分页直接实时查库。
const discoverRankingSize = 200

type DiscoverService struct {
	userRepo    repository.UserRepository
	feedRepo    repository.FeedRepository
	followRepo  repository.FollowRepository
	feedService *FeedService // 复用动态响应构建（作者/点赞态/转发原文）
}

func NewDiscoverService() *DiscoverService {
	return &DiscoverService{
		userRepo:    repository.NewUserRepository(models.DB),
		feedRepo:    repository.NewFeedRepository(models.DB),
		followRepo:  repository.NewFollowRepository(models.DB),
		feedService: NewFeedService(),
	}
}

// PopularUsers 热门用户列表：粉丝数降序，排除当前登录用户，携带关注态。
func (s *DiscoverService) PopularUsers(currentUserID uint, page, pageSize int) ([]models.UserResponse, int64, error) {
	offset := (page - 1) * pageSize

	rank := s.getRanking(cache.DiscoverKeyUsers, func() ([]uint, int64, error) {
		return s.userRepo.ListTopUserIDs(discoverRankingSize)
	})
	if rank != nil {
		// 缓存为全站排行，"排除本人"在读路径过滤，total 相应减 1
		ids := make([]uint, 0, len(rank.IDs))
		for _, id := range rank.IDs {
			if id != currentUserID {
				ids = append(ids, id)
			}
		}
		if offset < len(ids) {
			end := offset + pageSize
			if end > len(ids) {
				end = len(ids)
			}
			pageIDs := ids[offset:end]
			if users, ok := s.loadUsers(pageIDs); ok {
				following := s.followingSet(currentUserID)
				responses := make([]models.UserResponse, 0, len(users))
				for _, u := range users {
					resp := u.ToResponse()
					resp.IsFollowed = following[u.ID]
					responses = append(responses, resp)
				}
				return responses, rank.Total - 1, nil
			}
			// 详情回查失败：落到下方实时查询
		}
	}

	// 深页（超出 Top 200）或缓存不可用：实时查库
	users, total, err := s.userRepo.ListPopularUsers(currentUserID, page, pageSize)
	if err != nil {
		return nil, 0, err
	}
	following := s.followingSet(currentUserID)
	responses := make([]models.UserResponse, 0, len(users))
	for _, u := range users {
		resp := u.ToResponse()
		resp.IsFollowed = following[u.ID]
		responses = append(responses, resp)
	}
	return responses, total, nil
}

// PopularFeeds 热门动态列表：点赞数降序，含作者、当前用户点赞态与转发原文。
func (s *DiscoverService) PopularFeeds(currentUserID uint, page, pageSize int) ([]models.FeedResponse, int64, error) {
	offset := (page - 1) * pageSize

	rank := s.getRanking(cache.DiscoverKeyFeeds, func() ([]uint, int64, error) {
		return s.feedRepo.ListTopFeedIDs(discoverRankingSize)
	})
	if rank != nil && offset < len(rank.IDs) {
		end := offset + pageSize
		if end > len(rank.IDs) {
			end = len(rank.IDs)
		}
		pageIDs := rank.IDs[offset:end]
		if feeds, ok := s.loadFeeds(pageIDs); ok {
			return s.feedService.buildFeedResponses(feeds, currentUserID), rank.Total, nil
		}
		// 详情回查失败：落到下方实时查询
	}

	// 深页（超出 Top 200）或缓存不可用：实时查库
	feeds, total, err := s.feedRepo.ListPopularFeeds(page, pageSize)
	if err != nil {
		return nil, 0, err
	}
	return s.feedService.buildFeedResponses(feeds, currentUserID), total, nil
}

// loadUsers 按榜单顺序加载用户；回查失败返回 ok=false，调用方回落实时查询。
func (s *DiscoverService) loadUsers(ids []uint) ([]models.User, bool) {
	users, err := s.userRepo.ListByIDs(ids)
	if err != nil {
		return nil, false
	}
	return reorderByID(users, ids, func(u models.User) uint { return u.ID }), true
}

// loadFeeds 按榜单顺序加载动态；回查失败返回 ok=false，调用方回落实时查询。
func (s *DiscoverService) loadFeeds(ids []uint) ([]models.Feed, bool) {
	feeds, err := s.feedRepo.ListByIDs(ids)
	if err != nil {
		return nil, false
	}
	return reorderByID(feeds, ids, func(f models.Feed) uint { return f.ID }), true
}

// getRanking 读排行缓存；未命中时执行 loader 回源并回填缓存，Redis 或回源失败返回 nil（调用方走实时查询）。
func (s *DiscoverService) getRanking(key string, loader func() ([]uint, int64, error)) *cache.DiscoverRanking {
	if rank, err := cache.GetDiscoverRanking(key); err == nil {
		return rank
	}
	ids, total, err := loader()
	if err != nil {
		return nil
	}
	cache.SetDiscoverRanking(key, total, ids)
	return &cache.DiscoverRanking{Total: total, IDs: ids}
}

// followingSet 当前用户关注集合，用于批量填充 is_followed。
func (s *DiscoverService) followingSet(currentUserID uint) map[uint]bool {
	following := make(map[uint]bool)
	if follows, err := s.followRepo.ListFollowingAll(currentUserID); err == nil {
		for _, f := range follows {
			following[f.FollowedID] = true
		}
	}
	return following
}

// reorderByID 将批量查询结果按给定 ID 顺序重排（缓存命中路径必须保持榜单顺序）。
// 已被删除/查不到的 ID 会被跳过，页面可能少于 pageSize（TTL 窗口内的可接受偏差）。
func reorderByID[T any](items []T, ids []uint, idOf func(T) uint) []T {
	byID := make(map[uint]T, len(items))
	for _, item := range items {
		byID[idOf(item)] = item
	}
	ordered := make([]T, 0, len(ids))
	for _, id := range ids {
		if item, ok := byID[id]; ok {
			ordered = append(ordered, item)
		}
	}
	return ordered
}
