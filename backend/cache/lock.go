package cache

import (
	"time"
)

// AcquireLock 使用 SET NX PX 获取分布式锁。
func AcquireLock(key, token string, ttl time.Duration) (bool, error) {
	return RedisClient.SetNX(Ctx, key, token, ttl).Result()
}

// ReleaseLock 仅当 token 匹配时释放锁，避免误删他人锁。
func ReleaseLock(key, token string) error {
	const lua = `
if redis.call('GET', KEYS[1]) == ARGV[1] then
  return redis.call('DEL', KEYS[1])
end
return 0
`
	return RedisClient.Eval(Ctx, lua, []string{key}, token).Err()
}
