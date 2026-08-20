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
		key := fmt.Sprintf("tb:%s:ip:%s", prefix, ip)
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

// AllowTokenBucket 按任意 key 执行令牌桶限流，返回是否通过与建议等待时间。
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

var tokenBucketScript = redis.NewScript(`
local key = KEYS[1]
local now = tonumber(ARGV[1])
local rate = tonumber(ARGV[2])
local burst = tonumber(ARGV[3])
local expire = tonumber(ARGV[4])
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
