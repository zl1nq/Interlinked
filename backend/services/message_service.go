package services

import (
	"errors"
	"feed/models"
	"feed/realtime"
	"feed/repository"
	"strings"
	"time"
)

// MessageService 负责私信会话与消息读写流程。
type MessageService struct {
	messageRepo repository.MessageRepository
	userRepo    repository.UserRepository
}

type SendMessageRequest struct {
	ToUserID uint   `json:"to_user_id" binding:"required"`
	Content  string `json:"content" binding:"required,min=1,max=1000"`
}

type ConversationItem struct {
	User       models.UserResponse `json:"user"`
	LastMsg    string              `json:"last_msg"`
	LastTime   string              `json:"last_time"`
	Unread     int64               `json:"unread"`
	TargetID   uint                `json:"target_id"`
	TargetName string              `json:"target_name"`
}

func NewMessageService() *MessageService {
	return &MessageService{
		messageRepo: repository.NewMessageRepository(models.DB),
		userRepo:    repository.NewUserRepository(models.DB),
	}
}

// SendMessage 发送私信：校验收发双方与内容后写入消息表。
func (s *MessageService) SendMessage(fromUserID uint, req *SendMessageRequest) (*models.Message, error) {
	if fromUserID == 0 { //如果发送者ID为0，则返回请先登录事件
		return nil, errors.New("请先登录")
	}
	if req.ToUserID == 0 || req.ToUserID == fromUserID { //如果接收者ID为0或等于发送者ID，则返回接收方无效事件
		return nil, errors.New("接收方无效")
	}

	content := strings.TrimSpace(req.Content) //去除消息内容前后空格
	if content == "" {
		return nil, errors.New("消息内容不能为空") //发送消息内容不能为空事件
	}

	if _, err := s.userRepo.GetByID(req.ToUserID); err != nil { //获取接收者用户
		return nil, errors.New("接收方用户不存在") //发送接收者用户不存在事件
	}

	msg := &models.Message{ //创建消息
		FromUserID: fromUserID,
		ToUserID:   req.ToUserID, //设置接收者ID
		Content:    content,      //设置消息内容
		CreatedAt:  time.Now(),   //设置创建时间
	}
	if err := s.messageRepo.CreateWithConversations(msg); err != nil { //创建消息并更新会话
		return nil, errors.New("发送失败") //发送发送失败事件
	}

	// 实时推送：对方在线时立即收到新消息事件。
	realtime.PushToUser(req.ToUserID, realtime.MessageEvent{ //接收方：读取接收者接受的消息（写入消息 前端读取）
		Type: "message:new", //设置消息类型
		Data: msg,           //设置消息数据
	})

	return msg, nil //返回消息
}

// GetConversationMessages 获取会话消息，并将“对方->当前用户”的未读消息标记为已读。
func (s *MessageService) GetConversationMessages(currentUserID, targetUserID uint, page, pageSize int) ([]models.Message, int64, error) {
	if currentUserID == 0 || targetUserID == 0 { //如果当前用户ID或目标用户ID为0，则返回参数错误事件
		return nil, 0, errors.New("参数错误")
	}
	if _, err := s.userRepo.GetByID(targetUserID); err != nil {
		return nil, 0, errors.New("接收方用户不存在")
	}
	list, total, err := s.messageRepo.GetByConversation(currentUserID, targetUserID, page, pageSize) //获取会话消息
	if err != nil {
		return nil, 0, err //发送获取会话消息失败事件
	}
	if total > 0 {
		_ = s.messageRepo.EnsureConversationFromHistory(currentUserID, targetUserID)
	}
	_ = s.messageRepo.MarkReadFromTo(targetUserID, currentUserID) //标记已读
	return s.enrichMessageUsers(list), total, nil                 //返回会话消息
}

func (s *MessageService) enrichMessageUsers(list []models.Message) []models.Message {
	for i := range list {
		if list[i].FromUser.ID == 0 {
			if user, err := s.userRepo.GetByID(list[i].FromUserID); err == nil {
				list[i].FromUser = *user
			}
		}
		if list[i].ToUser.ID == 0 {
			if user, err := s.userRepo.GetByID(list[i].ToUserID); err == nil {
				list[i].ToUser = *user
			}
		}
	}
	return list
}

// 获取会话列表
func (s *MessageService) GetConversationList(currentUserID uint, page, pageSize int) ([]ConversationItem, int64, error) {
	if currentUserID == 0 {
		return nil, 0, errors.New("请先登录") //发送请先登录事件
	}

	conversations, total, err := s.messageRepo.ListConversations(currentUserID, page, pageSize)
	if err != nil {
		return nil, 0, err //发送获取会话列表失败事件
	}

	items := make([]ConversationItem, 0, len(conversations))
	for _, conv := range conversations {
		items = append(items, ConversationItem{
			User:       conv.TargetUser.ToResponse(),
			LastMsg:    conv.LastMessageContent,
			LastTime:   conv.LastMessageAt.Format(time.RFC3339),
			Unread:     conv.UnreadCount,
			TargetID:   conv.TargetUserID,
			TargetName: conv.TargetUser.Nickname,
		})
	}

	return items, total, nil //返回会话列表
}
