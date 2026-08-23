package services

import (
	"context"
	"encoding/json"
	"feed/config"
	"feed/models"
	"feed/mq"
	"feed/repository"
	"log"
	"time"
)

// OutboxService 扫描可靠事件表并投递到下游异步组件。
type OutboxService struct {
	repo repository.OutboxRepository
}

func NewOutboxService() *OutboxService {
	return &OutboxService{repo: repository.NewOutboxRepository(models.DB)}
}

// Start Outbox 模式 的轮询驱动核心，负责定期扫描数据库中的待投递事件并推送到下游
func (s *OutboxService) Start(ctx context.Context) {
	ticker := time.NewTicker(time.Duration(config.AppConfig.Outbox.PollIntervalMS) * time.Millisecond)
	defer ticker.Stop()

	for {
		if err := s.dispatchDueEvents(); err != nil {
			log.Printf("outbox dispatch failed: %v", err)
		}

		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

// dispatchDueEvents 批处理调度器，管理事件的生命周期状态流转
func (s *OutboxService) dispatchDueEvents() error {
	events, err := s.repo.ListDue(config.AppConfig.Outbox.BatchSize)
	if err != nil {
		return err
	}
	for _, event := range events {
		// 发送失败时的处理逻辑
		if err := s.dispatchEvent(event); err != nil {
			nextRetryCount := event.RetryCount + 1
			lastErr := truncateError(err)
			// 达到上限 → 标记为死信
			if nextRetryCount >= config.AppConfig.Outbox.MaxRetries {
				if markErr := s.repo.MarkDead(event.ID, nextRetryCount, lastErr); markErr != nil {
					log.Printf("mark outbox event dead failed, id=%d err=%v", event.ID, markErr)
				}
				log.Printf("outbox event moved to dead, id=%d type=%s retries=%d err=%v", event.ID, event.EventType, nextRetryCount, err)
				continue
			}
			// 未达上限 → 计算下次重试时间（指数退避）
			nextRetry := time.Now().Add(backoffDuration(nextRetryCount))
			if markErr := s.repo.MarkFailed(event.ID, nextRetryCount, lastErr, nextRetry); markErr != nil {
				log.Printf("mark outbox event failed, id=%d err=%v", event.ID, markErr)
			}
			continue
		}
		// 事件投递成功 → 标记为已发送
		if err := s.repo.MarkSent(event.ID); err != nil {
			log.Printf("mark outbox event sent failed, id=%d err=%v", event.ID, err)
		}
	}
	return nil
}

// dispatchEvent Outbox 事件的实际投递逻辑——根据事件类型将载荷数据路由到对应的下游处理器（消息队列或清理逻辑）
func (s *OutboxService) dispatchEvent(event models.OutboxEvent) error {
	switch event.EventType {
	case models.OutboxEventTypeFeedPublished:
		var payload struct {
			FeedID   uint `json:"feed_id"`
			AuthorID uint `json:"author_id"`
		}
		// 将存储在 event.Payload 中的 JSON 反序列化为匿名结构体 payload
		if err := json.Unmarshal([]byte(event.Payload), &payload); err != nil {
			return err
		}
		//	调用 mq.PublishFeed 将动态投递给消息队列
		return mq.PublishFeed(payload.FeedID, payload.AuthorID)
	case models.OutboxEventTypeFeedDeleted:
		var payload struct {
			FeedID   uint `json:"feed_id"`
			AuthorID uint `json:"author_id"`
		}
		if err := json.Unmarshal([]byte(event.Payload), &payload); err != nil {
			return err
		}
		// 调用 CleanupDeletedFeed 执行级联清理（如删除点赞、评论、时间线条目等关联数据）
		return CleanupDeletedFeed(payload.FeedID, payload.AuthorID)
	default:
		log.Printf("unknown outbox event type ignored, id=%d type=%s", event.ID, event.EventType)
		return nil
	}
}

func backoffDuration(retryCount int) time.Duration {
	if retryCount < 1 { // 1. 参数保护：如果重试次数小于1，设为1
		retryCount = 1
	}
	// 2. 计算指数退避时间
	// min(retryCount-1, 8) 限制最大位移为8（即最大2^8=256秒）
	// 1<<n 表示 2 的 n 次方
	d := time.Duration(1<<min(retryCount-1, 8)) * time.Second
	// 3. 获取配置的最大退避时间
	maxBackoff := time.Duration(config.AppConfig.Outbox.MaxBackoffMS) * time.Millisecond
	// 4. 不能超过配置的最大值
	if d > maxBackoff {
		return maxBackoff
	}
	return d
}

func truncateError(err error) string {
	if err == nil {
		return ""
	}
	msg := err.Error()
	if len(msg) > 1000 {
		return msg[:1000]
	}
	return msg
}
