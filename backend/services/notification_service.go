package services

import (
	"errors"
	"feed/models"
	"feed/repository"
	"log"
	"time"
)

// NotificationService 负责通知中心业务编排。
type NotificationService struct {
	notificationRepo repository.NotificationRepository
}

type NotificationItem struct {
	ID        uint                `json:"id"`
	Type      string              `json:"type"`
	TargetID  uint                `json:"target_id"`
	Content   string              `json:"content"`
	IsRead    bool                `json:"is_read"`
	CreatedAt string              `json:"created_at"`
	Actor     models.UserResponse `json:"actor"`
}

func NewNotificationService() *NotificationService {
	return &NotificationService{notificationRepo: repository.NewNotificationRepository(models.DB)}
}

// CreateLikeNotification 创建点赞通知。
func (s *NotificationService) CreateLikeNotification(actorID, receiverID, feedID uint) {
	if actorID == 0 || receiverID == 0 || actorID == receiverID {
		return
	}
	_ = s.notificationRepo.Create(&models.Notification{
		UserID:   receiverID,
		ActorID:  actorID,
		Type:     models.NotificationTypeLike,
		TargetID: feedID,
		Content:  "赞了你的动态",
	})
}

// CreateCommentNotification 创建评论通知。
func (s *NotificationService) CreateCommentNotification(actorID, receiverID, feedID uint, content string) {
	if actorID == 0 || receiverID == 0 || actorID == receiverID {
		return
	}
	_ = s.notificationRepo.Create(&models.Notification{
		UserID:   receiverID,
		ActorID:  actorID,
		Type:     models.NotificationTypeComment,
		TargetID: feedID,
		Content:  "评论了你的动态: " + content,
	})
}

// CreateFollowNotification 创建关注通知。
func (s *NotificationService) CreateFollowNotification(actorID, receiverID uint) {
	if actorID == 0 || receiverID == 0 || actorID == receiverID {
		return
	}
	_ = s.notificationRepo.Create(&models.Notification{
		UserID:  receiverID,
		ActorID: actorID,
		Type:    models.NotificationTypeFollow,
		Content: "关注了你",
	})
}

// ListNotifications 获取通知列表及未读数。
func (s *NotificationService) ListNotifications(userID uint, page, pageSize int) ([]NotificationItem, int64, int64, error) {
	list, total, err := s.notificationRepo.ListByUser(userID, page, pageSize) //获取通知列表
	if err != nil {
		return nil, 0, 0, err //发送获取通知列表失败事件
	}
	unread, _ := s.notificationRepo.CountUnread(userID) //获取未读通知数

	result := make([]NotificationItem, 0, len(list)) //创建通知列表
	for _, n := range list {
		result = append(result, NotificationItem{ //添加通知列表
			ID:        n.ID,
			Type:      n.Type,
			TargetID:  n.TargetID, //设置目标ID
			Content:   n.Content,  //设置内容
			IsRead:    n.IsRead,   //设置是否已读
			CreatedAt: n.CreatedAt.Format(time.RFC3339),
			Actor:     n.Actor.ToResponse(),
		})
	}
	return result, total, unread, nil
}

// MarkAllRead 全部标记已读。
func (s *NotificationService) MarkAllRead(userID uint) error {
	return s.notificationRepo.MarkAllRead(userID) //标记已读
}

// DeleteNotification 删除单条通知（无论是否已读），仅限本人通知。
// 返回删除后的最新未读数，便于前端直接刷新未读徽标。
func (s *NotificationService) DeleteNotification(userID, notificationID uint) (int64, error) {
	rows, err := s.notificationRepo.DeleteByUserAndID(userID, notificationID)
	if err != nil {
		return 0, err
	}
	if rows == 0 {
		return 0, errors.New("通知不存在")
	}
	unread, err := s.notificationRepo.CountUnread(userID)
	if err != nil {
		return 0, err
	}
	return unread, nil
}

// ClearReadNotifications 一键清空全部已读通知（未读保留），返回删除条数。
func (s *NotificationService) ClearReadNotifications(userID uint) (int64, error) {
	return s.notificationRepo.DeleteReadByUser(userID)
}

// RetractNotification 撤回一条行为通知（取消点赞/删评论/取关）。
// 与创建通知一样 fire-and-forget：失败仅记日志，不影响主流程。
func (s *NotificationService) RetractNotification(actorID, receiverID, targetID uint, notificationType string) {
	if actorID == 0 || receiverID == 0 {
		return
	}
	if _, err := s.notificationRepo.DeleteByActorTarget(actorID, receiverID, targetID, notificationType); err != nil {
		log.Printf("retract notification failed: actor=%d receiver=%d target=%d type=%s err=%v",
			actorID, receiverID, targetID, notificationType, err)
	}
}

// CleanupFeedNotifications 动态删除时的级联清理：删除指向该动态的全部点赞/评论通知。
func (s *NotificationService) CleanupFeedNotifications(feedID uint) error {
	_, err := s.notificationRepo.DeleteByTargetFeed(feedID)
	return err
}
