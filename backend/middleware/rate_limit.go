package middleware

import (
	"feed/cache"
	"feed/utils"
	"fmt"
	"math"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
)

//实现了基于 Redis 令牌桶算法 的接口限流中间件，用于保护后端接口免受高频请求的冲击。

// RateLimitByIP 按 IP 做令牌桶限流。
// rate 表示每秒补充令牌数，burst 表示桶容量。
func RateLimitByIP(prefix string, rate float64, burst int) gin.HandlerFunc {
	return func(c *gin.Context) {
		if rate <= 0 || burst <= 0 {
			c.Next()
			return
		}
		ip := c.ClientIP()
		if ip == "" {
			ip = "unknown"
		}
		key := fmt.Sprintf("tb:%s:ip:%s", prefix, ip)                 //生成令牌桶键名
		pass, retryAfter, err := allowByTokenBucket(key, rate, burst) //获取令牌
		if err != nil {                                               //发送获取令牌失败事件
			c.Next() //继续执行
			return
		}
		if !pass { //如果令牌不足，则返回操作过于频繁事件
			utils.Error(c, 429, "请求过于频繁，请稍后再试")
			c.Header("Retry-After", strconv.Itoa(retryAfter)) //设置重试时间
			c.Abort()
			return //终止请求
		}
		c.Next()
	}
}

// RateLimitByUser 按 user_id 做令牌桶限流。
// rate 表示每秒补充令牌数，burst 表示桶容量。
// 针对特定资源（如视频）的访问限制
func RateLimitByUser(prefix string, rate float64, burst int) gin.HandlerFunc {
	return func(c *gin.Context) {
		if rate <= 0 || burst <= 0 {
			c.Next()
			return
		}
		userID := GetCurrentUserID(c)
		if userID == 0 {
			c.Next()
			return
		}
		key := fmt.Sprintf("tb:%s:user:%d", prefix, userID)
		pass, retryAfter, err := allowByTokenBucket(key, rate, burst) //获取令牌
		if err != nil {                                               //发送获取令牌失败事件
			c.Next() //继续执行
			return
		}
		if !pass { //如果令牌不足，则返回操作过于频繁事件
			utils.Error(c, 429, "操作过于频繁，请稍后再试")
			c.Header("Retry-After", strconv.Itoa(retryAfter)) //设置重试时间
			c.Abort()
			return //终止请求
		}
		c.Next()
	}
}

// RateLimitByFeedFromContext 按 path 中的 feed_id 做令牌桶限流。
// rate 表示每秒补充令牌数，burst 表示桶容量。
func RateLimitByFeedFromContext(prefix string, rate float64, burst int) gin.HandlerFunc {
	return func(c *gin.Context) {
		if rate <= 0 || burst <= 0 {
			c.Next()
			return
		}
		feedID, err := strconv.ParseUint(c.Param("id"), 10, 64)
		if err != nil || feedID == 0 {
			c.Next()
			return
		}
		key := fmt.Sprintf("tb:%s:feed:%d", prefix, feedID)
		pass, retryAfter, err := allowByTokenBucket(key, rate, burst)
		if err != nil {
			c.Next()
			return
		}
		if !pass {
			utils.Error(c, 429, "视频点赞过于频繁，请稍后再试")
			c.Header("Retry-After", strconv.Itoa(retryAfter))
			c.Abort()
			return
		}
		c.Next()
	}
}

// AllowTokenBucket 按任意 key 执行令牌桶限流，返回是否通过与建议等待时间。对外暴露的任意 Key 限流接口
func AllowTokenBucket(key string, rate float64, burst int) (bool, int, error) {
	return allowByTokenBucket(key, rate, burst)
}

// allowByTokenBucket 使用 Redis Lua 原子实现令牌桶。
func allowByTokenBucket(key string, rate float64, burst int) (bool, int, error) {
	if rate <= 0 || burst <= 0 {
		return true, 0, nil
	}
	expireSec := max(int(math.Ceil(float64(burst)/rate))*2, 2)

	result, err := tokenBucketScript.Run(cache.Ctx, cache.RedisClient, []string{key}, time.Now().UnixMilli(), rate, burst, expireSec).Result()
	if err != nil {
		return true, 0, err
	}

	values, ok := result.([]any)
	if !ok || len(values) < 2 {
		return true, 0, nil
	}
	allowed := parseLuaInt(values[0]) == 1
	retryAfter := parseLuaInt(values[1])
	return allowed, retryAfter, nil
}

/*
核心：allowByTokenBucket + Lua 脚本（第 107-154 行）
使用 Redis Lua 脚本原子性地执行令牌桶算法，两个 Key 存储状态：

tokensKey：当前剩余令牌数
tsKey：上次执行时间戳
算法流程：

计算时间差 delta，按 delta * rate 补充令牌（不超过 burst）
若 tokens < 1（桶空），计算需要等待的时间 retryAfter，返回 {0, retryAfter}
若有令牌，扣减 1 个令牌，返回 {1, 0}
每次写入都设置过期时间 expireSec = max(ceil(burst/rate)*2, 2)，避免僵尸 Key 堆积
*/
var tokenBucketScript = redis.NewScript(`
local key = KEYS[1]          -- Redis Key 前缀
local now = tonumber(ARGV[1]) -- 当前时间戳（毫秒）
local rate = tonumber(ARGV[2]) -- 令牌补充速率
local burst = tonumber(ARGV[3]) -- 桶容量
local expire = tonumber(ARGV[4]) -- Key 过期时间（秒）
local tokensKey = key .. ':tokens'
local tsKey = key .. ':ts'
local tokens = tonumber(redis.call('GET', tokensKey))
local lastTs = tonumber(redis.call('GET', tsKey))
if tokens == nil then tokens = burst end
if lastTs == nil then lastTs = now end
if now > lastTs then
  local delta = (now - lastTs) / 1000
  tokens = math.min(burst, tokens + delta * rate)
end
if tokens < 1 then
  local retryAfter = math.ceil((1 - tokens) / rate)
  if retryAfter < 1 then retryAfter = 1 end
  redis.call('SET', tokensKey, tokens, 'EX', expire)
  redis.call('SET', tsKey, now, 'EX', expire)
  return {0, retryAfter}
end
tokens = tokens - 1
redis.call('SET', tokensKey, tokens, 'EX', expire)
redis.call('SET', tsKey, now, 'EX', expire)
return {1, 0}
`)

// 兼容 Redis 返回的不同类型（int64/int/string）
func parseLuaInt(v any) int {
	switch val := v.(type) {
	case int64:
		return int(val)
	case int:
		return val
	case string:
		n, _ := strconv.Atoi(val)
		return n
	default:
		return 0
	}
}
