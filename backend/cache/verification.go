// verification.go 邮箱验证码的 Redis 存储与防刷，以及 token 版本缓存。
// 验证码相关 Key：
// - vcode:{scene}:{email}         Hash{code, attempts}，验证码本体
// - vcode:cd:{scene}:{email}      发送冷却标记（SETNX + TTL）
// - vcode:dl:{scene}:{email}:{day} 当日已发送计数
package cache

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"strconv"
	"time"

	"github.com/go-redis/redis/v8"
)

const (
	keyVerifyCode    = "vcode:%s:%s"         // 验证码 Hash
	keyVerifyCodeCD  = "vcode:cd:%s:%s"      // 发送冷却
	keyVerifyCodeDay = "vcode:dl:%s:%s:%s"   // 每日限额
	KeyTokenVersion  = "tv:%d"               // token 版本缓存
	keyPendingReg    = "pending:register:%s" // 注册待激活资料
)

// ErrCodeCooldown / ErrCodeDailyLimit / ErrCodeTooManyAttempts 由调用方转换为友好文案。
var (
	ErrCodeCooldown        = errors.New("发送过于频繁，请稍后再试")
	ErrCodeDailyLimit      = errors.New("今日发送次数已达上限，请明天再试")
	ErrCodeTooManyAttempts = errors.New("验证码错误次数过多，请重新获取")
	ErrCodeExpired         = errors.New("验证码已过期或未发送")
	ErrCodeMismatch        = errors.New("验证码错误")
)

// SaveVerificationCode 生成 6 位数字验证码并写入 Redis。
// 返回生成的验证码；写失败返回 error（调用方应让发送流程失败）。
func SaveVerificationCode(scene, email string, ttl time.Duration) (string, error) {
	code, err := generateNumericCode(6)
	if err != nil {
		return "", err
	}
	key := fmt.Sprintf(keyVerifyCode, scene, email)
	if err := RedisClient.HSet(Ctx, key, map[string]any{
		"code":     code,
		"attempts": 0,
	}).Err(); err != nil {
		return "", err
	}
	if err := RedisClient.Expire(Ctx, key, ttl).Err(); err != nil {
		return "", err
	}
	return code, nil
}

// CheckSendQuota 校验发送冷却与每日限额，通过时写入冷却标记并累加当日计数。
// 任何一项不满足都返回对应错误（不计数、不落冷却）。
func CheckSendQuota(scene, email string, cooldown time.Duration, dailyLimit int) error {
	cdKey := fmt.Sprintf(keyVerifyCodeCD, scene, email)
	ok, err := RedisClient.SetNX(Ctx, cdKey, "1", cooldown).Result()
	if err != nil {
		return err
	}
	if !ok {
		return ErrCodeCooldown
	}

	dayKey := fmt.Sprintf(keyVerifyCodeDay, scene, email, time.Now().Format("20060102"))
	cnt, err := RedisClient.Incr(Ctx, dayKey).Result()
	if err != nil {
		return err
	}
	// 当日计数 key 到次日零点过期即可，简化为 24h。
	if cnt == 1 {
		_ = RedisClient.Expire(Ctx, dayKey, 24*time.Hour).Err()
	}
	if cnt > int64(dailyLimit) {
		return ErrCodeDailyLimit
	}
	return nil
}

// VerifyVerificationCode 校验验证码：错误累计 attempts，成功或超限后删除。
func VerifyVerificationCode(scene, email, code string, maxAttempts int) error {
	key := fmt.Sprintf(keyVerifyCode, scene, email)
	data, err := RedisClient.HGetAll(Ctx, key).Result()
	if err != nil {
		return err
	}
	if len(data) == 0 {
		return ErrCodeExpired
	}
	if code != data["code"] {
		attempts, _ := strconv.Atoi(data["attempts"])
		attempts++
		_ = RedisClient.HSet(Ctx, key, "attempts", attempts).Err()
		if attempts >= maxAttempts {
			_ = RedisClient.Del(Ctx, key).Err()
			return ErrCodeTooManyAttempts
		}
		return ErrCodeMismatch
	}
	return RedisClient.Del(Ctx, key).Err()
}

func generateNumericCode(n int) (string, error) {
	// rand.Int 上界为开区间，取 10^n 保证均匀覆盖 [0, 10^n)
	bound := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(n)), nil)
	val, err := rand.Int(rand.Reader, bound)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%0*d", n, val.Int64()), nil
}

// GetTokenVersion 读取 token 版本缓存；未命中返回 (0, redis.Nil)。
func GetTokenVersion(userID uint) (int, error) {
	key := fmt.Sprintf(KeyTokenVersion, userID)
	v, err := RedisClient.Get(Ctx, key).Int()
	if err == redis.Nil {
		return 0, err
	}
	return v, err
}

// SetTokenVersion 缓存 token 版本，短 TTL，用于鉴权中间件免查库。
func SetTokenVersion(userID uint, version int) {
	key := fmt.Sprintf(KeyTokenVersion, userID)
	_ = RedisClient.Set(Ctx, key, version, 10*time.Minute).Err()
}

// InvalidateTokenVersion 改密/重置后调用，强制下次请求回源 DB 读新版本。
func InvalidateTokenVersion(userID uint) {
	_ = RedisClient.Del(Ctx, fmt.Sprintf(KeyTokenVersion, userID)).Err()
}

// PendingRegistration 注册第一步的待激活资料（密码只存 bcrypt 摘要）。
type PendingRegistration struct {
	Username      string `json:"username"`
	PasswordHash  string `json:"password_hash"`
	Nickname      string `json:"nickname"`
	Email         string `json:"email"`
	EmailVerified bool   `json:"email_verified"`
}

// SavePendingRegistration 暂存注册资料；同邮箱重复提交覆盖旧资料。
func SavePendingRegistration(email string, p *PendingRegistration, ttl time.Duration) error {
	b, err := json.Marshal(p)
	if err != nil {
		return err
	}
	return RedisClient.Set(Ctx, fmt.Sprintf(keyPendingReg, email), b, ttl).Err()
}

// GetPendingRegistration 读取待激活资料；不存在返回 redis.Nil。
func GetPendingRegistration(email string) (*PendingRegistration, error) {
	raw, err := RedisClient.Get(Ctx, fmt.Sprintf(keyPendingReg, email)).Result()
	if err != nil {
		return nil, err
	}
	var p PendingRegistration
	if err := json.Unmarshal([]byte(raw), &p); err != nil {
		return nil, err
	}
	return &p, nil
}

// DeletePendingRegistration 注册完成或资料作废后清理。
func DeletePendingRegistration(email string) {
	_ = RedisClient.Del(Ctx, fmt.Sprintf(keyPendingReg, email)).Err()
}
