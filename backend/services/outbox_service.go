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

func (s *OutboxService) dispatchDueEvents() error {
	events, err := s.repo.ListDue(config.AppConfig.Outbox.BatchSize)
	if err != nil {
		return err
	}
	for _, event := range events {
		if err := s.dispatchEvent(event); err != nil {
			nextRetryCount := event.RetryCount + 1
			lastErr := truncateError(err)
			if nextRetryCount >= config.AppConfig.Outbox.MaxRetries {
				if markErr := s.repo.MarkDead(event.ID, nextRetryCount, lastErr); markErr != nil {
					log.Printf("mark outbox event dead failed, id=%d err=%v", event.ID, markErr)
				}
				log.Printf("outbox event moved to dead, id=%d type=%s retries=%d err=%v", event.ID, event.EventType, nextRetryCount, err)
				continue
			}
			nextRetry := time.Now().Add(backoffDuration(nextRetryCount))
			if markErr := s.repo.MarkFailed(event.ID, nextRetryCount, lastErr, nextRetry); markErr != nil {
				log.Printf("mark outbox event failed, id=%d err=%v", event.ID, markErr)
			}
			continue
		}
		if err := s.repo.MarkSent(event.ID); err != nil {
			log.Printf("mark outbox event sent failed, id=%d err=%v", event.ID, err)
		}
	}
	return nil
}

func (s *OutboxService) dispatchEvent(event models.OutboxEvent) error {
	switch event.EventType {
	case models.OutboxEventTypeFeedPublished:
		var payload struct {
			FeedID   uint `json:"feed_id"`
			AuthorID uint `json:"author_id"`
		}
		if err := json.Unmarshal([]byte(event.Payload), &payload); err != nil {
			return err
		}
		return mq.PublishFeed(payload.FeedID, payload.AuthorID)
	case models.OutboxEventTypeFeedDeleted:
		var payload struct {
			FeedID   uint `json:"feed_id"`
			AuthorID uint `json:"author_id"`
		}
		if err := json.Unmarshal([]byte(event.Payload), &payload); err != nil {
			return err
		}
		return CleanupDeletedFeed(payload.FeedID, payload.AuthorID)
	default:
		log.Printf("unknown outbox event type ignored, id=%d type=%s", event.ID, event.EventType)
		return nil
	}
}

func backoffDuration(retryCount int) time.Duration {
	if retryCount < 1 {
		retryCount = 1
	}
	d := time.Duration(1<<min(retryCount-1, 8)) * time.Second
	maxBackoff := time.Duration(config.AppConfig.Outbox.MaxBackoffMS) * time.Millisecond
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
