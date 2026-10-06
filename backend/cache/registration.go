package cache

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/go-redis/redis/v8"
)

var ErrRegistrationExpired = errors.New("注册凭证无效、已过期或已使用，请重新验证邮箱")

const registrationKey = "registration:ticket:"

// 验证、错误计数、验证码消费和凭证签发在同一脚本内完成，防止并发换取多个凭证。
var exchangeRegistrationCode = redis.NewScript(`
local code = redis.call('HGET', KEYS[1], 'code')
if not code then return 1 end
if code ~= ARGV[1] then
 local attempts = redis.call('HINCRBY', KEYS[1], 'attempts', 1)
 if attempts >= tonumber(ARGV[2]) then redis.call('DEL', KEYS[1]); return 2 end
 return 3
end
redis.call('SET', KEYS[2], ARGV[3], 'PX', ARGV[4])
redis.call('DEL', KEYS[1])
return 0
`)

func VerifyRegistrationEmail(email, code string, maxAttempts int, ttl time.Duration) (string, error) {
	if ttl <= 0 {
		return "", errors.New("注册凭证有效期配置错误")
	}
	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		return "", err
	}
	token := hex.EncodeToString(secret)
	result, err := exchangeRegistrationCode.Run(Ctx, RedisClient,
		[]string{fmt.Sprintf(keyVerifyCode, "register", email), registrationKey + token},
		code, maxAttempts, email, ttl.Milliseconds()).Int()
	if err != nil {
		return "", err
	}
	switch result {
	case 1:
		return "", ErrCodeExpired
	case 2:
		return "", ErrCodeTooManyAttempts
	case 3:
		return "", ErrCodeMismatch
	}
	return token, nil
}

func CheckRegistrationTicket(token, email string) error {
	if len(token) != 64 {
		return ErrRegistrationExpired
	}
	bound, err := RedisClient.Get(Ctx, registrationKey+token).Result()
	if err == redis.Nil {
		return ErrRegistrationExpired
	}
	if err != nil {
		return err
	}
	if bound != email {
		return ErrRegistrationExpired
	}
	return nil
}

var consumeRegistrationTicket = redis.NewScript(`
if redis.call('GET', KEYS[1]) ~= ARGV[1] then return 0 end
return redis.call('DEL', KEYS[1])
`)

func ConsumeRegistrationTicket(token, email string) error {
	if len(token) != 64 {
		return ErrRegistrationExpired
	}
	result, err := consumeRegistrationTicket.Run(Ctx, RedisClient, []string{registrationKey + token}, email).Int()
	if err != nil {
		return err
	}
	if result != 1 {
		return ErrRegistrationExpired
	}
	return nil
}
