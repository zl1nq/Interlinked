// verification_service.go 邮箱验证码的业务编排：发送配额校验、验证码生成与校验、邮件投递。
// 场景（scene）决定 Redis Key 与邮件文案，互不串用：
// - register          注册确认（发送目标=待注册邮箱）
// - change_password   已登录修改密码（发送目标=当前用户邮箱）
// - forgot_password   未登录找回密码（发送目标=预留邮箱）
// - change_email      绑定/更换邮箱（发送目标=新邮箱）
package services

import (
	"fmt"
	"strings"
	"time"

	"feed/cache"
	"feed/config"
	"feed/mail"
)

const (
	EmailSceneRegister       = "register"
	EmailSceneChangePassword = "change_password"
	EmailSceneForgotPassword = "forgot_password"
	EmailSceneChangeEmail    = "change_email"
)

// VerificationService 验证码域业务编排。
type VerificationService struct{}

func NewVerificationService() *VerificationService {
	return &VerificationService{}
}

// SendCode 发送验证码到指定邮箱。
// 配额失败（冷却/超日限）返回业务错误；Redis 或邮件发送失败原样上抛。
func (s *VerificationService) SendCode(scene, email string) error {
	email = normalizeEmail(email)
	cfg := config.AppConfig.Email

	if err := cache.CheckSendQuota(scene, email,
		time.Duration(cfg.SendCooldownSec)*time.Second, cfg.DailyLimit); err != nil {
		return err
	}

	code, err := cache.SaveVerificationCode(scene, email,
		time.Duration(cfg.CodeTTLMin)*time.Minute)
	if err != nil {
		return err
	}

	subject, body := buildCodeMail(scene, code, cfg.CodeTTLMin)
	if err := mail.Default().Send(email, subject, body); err != nil {
		return fmt.Errorf("发送邮件失败: %w", err)
	}
	return nil
}

// VerifyCode 校验验证码。成功即销毁（一次性），失败累计错误次数。
func (s *VerificationService) VerifyCode(scene, email, code string) error {
	email = normalizeEmail(email)
	return cache.VerifyVerificationCode(scene, email, code, config.AppConfig.Email.CodeMaxAttempts)
}

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

func buildCodeMail(scene, code string, ttlMin int) (string, string) {
	var purpose string
	switch scene {
	case EmailSceneRegister:
		purpose = "完成注册"
	case EmailSceneChangePassword:
		purpose = "修改密码"
	case EmailSceneForgotPassword:
		purpose = "重置密码"
	case EmailSceneChangeEmail:
		purpose = "绑定新邮箱"
	default:
		purpose = "身份验证"
	}
	subject := fmt.Sprintf("【Interlinked】你的验证码：%s", code)
	body := fmt.Sprintf(
		"你正在%s。\n\n验证码：%s\n有效期：%d 分钟\n\n如果这不是你的操作，请忽略本邮件。",
		purpose, code, ttlMin,
	)
	return subject, body
}
