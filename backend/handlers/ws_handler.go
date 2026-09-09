package handlers

import (
	"encoding/json"
	"feed/config"
	"feed/middleware"
	"feed/realtime"
	"feed/services"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

// WSHandler 负责 WebSocket 连接升级、统一鉴权与消息协议处理。
type WSHandler struct {
	messageService *services.MessageService
}

func NewWSHandler(messageService *services.MessageService) *WSHandler {
	return &WSHandler{messageService: messageService}
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return middleware.IsAllowedOrigin(r.Header.Get("Origin"))
	},
}

const (
	wsReadTimeout     = 70 * time.Second
	wsPongWait        = 70 * time.Second
	wsPingInterval    = 25 * time.Second
	wsWriteWait       = 10 * time.Second
	maxMessageContent = 1000
)

type wsInboundEvent struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data"`
}

type wsSendMessageData struct {
	ToUserID    uint   `json:"to_user_id"`
	Content     string `json:"content"`
	ClientMsgID string `json:"client_msg_id"`
}

type wsConversationsData struct {
	ReqID    string `json:"req_id"`
	Page     int    `json:"page"`
	PageSize int    `json:"page_size"`
}

type wsHistoryData struct {
	ReqID      string `json:"req_id"`
	TargetUser uint   `json:"target_user_id"`
	Page       int    `json:"page"`
	PageSize   int    `json:"page_size"`
}

// MessageWS 建立私信实时通道。
// GET /ws/messages?token=xxx
func (h *WSHandler) MessageWS(c *gin.Context) {
	claims, err := middleware.ParseTokenFromRequest(c) //解析token
	//============Token 验证===============
	//发送无效token事件
	if err != nil || claims == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"message": "invalid token"})
		return
	}
	//获取当前用户ID
	userID := middleware.GetCurrentUserID(c)
	if userID == 0 {
		//发送未授权事件
		c.JSON(http.StatusUnauthorized, gin.H{"message": "unauthorized"})
		return
	}
	//检查 Token 是否过期
	if claims.ExpiresAt == nil || claims.ExpiresAt.Time.Before(time.Now()) {
		//发送令牌过期事件
		c.JSON(http.StatusUnauthorized, gin.H{"message": "token expired"})
		return
	}
	//============升级 WebSocket 连接==========
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}

	//============设置超时和 Ping/Pong==========
	//设置读取超时时间
	_ = conn.SetReadDeadline(time.Now().Add(wsReadTimeout))
	/*
		设置 Pong 处理器
		收到客户端的 Pong 响应时，刷新读取超时时间
		实现心跳保活机制
	*/
	conn.SetPongHandler(func(string) error {
		return conn.SetReadDeadline(time.Now().Add(wsPongWait)) //设置pong等待时间
	})

	//============注册客户端==========
	//创建客户端
	client := &realtime.Client{
		UserID:    userID,
		Conn:      conn,
		ExpiresAt: claims.ExpiresAt.Time,
	}
	//注册连接
	realtime.RegisterConn(client)
	defer func() {
		realtime.UnregisterConn(client)
		_ = conn.Close()
	}()

	//============启动心跳==========
	stopPing := make(chan struct{})  //创建停止ping通道
	go h.keepAlive(client, stopPing) //启动心跳检测

	//============主循环处理消息==========
	for {
		//如果令牌过期，则关闭连接
		if time.Now().After(client.ExpiresAt) {
			_ = client.WriteJSON(realtime.MessageEvent{Type: "auth:expired", Data: gin.H{"message": "token expired"}})                                                    //发送令牌过期事件
			_ = client.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.ClosePolicyViolation, "token expired"), time.Now().Add(2*time.Second)) //发送关闭连接事件
			break
		}
		//发送方：读取发送者发送的消息（读取前端发送的消息）
		_, payload, err := conn.ReadMessage()
		if err != nil {
			break //如果读取消息失败，则关闭连接
		}
		//----------限流控制（令牌桶）----------
		wsCfg := config.AppConfig.WS.SendMessage
		if pass, retryAfter, _ := middleware.AllowTokenBucket("tb:ws:send:user:"+strconv.FormatUint(uint64(userID), 10), wsCfg.Rate, wsCfg.Burst); !pass {
			_ = client.WriteJSON(realtime.MessageEvent{Type: "message:error", Data: gin.H{"message": "发送过于频繁，请稍后再试", "retry_after": retryAfter}})
			continue //如果发送过于频繁，则发送错误事件
		}
		//----------解析消息事件----------
		var event wsInboundEvent
		if err := json.Unmarshal(payload, &event); err != nil {
			_ = client.WriteJSON(realtime.MessageEvent{Type: "message:error", Data: gin.H{"message": "invalid payload"}})
			continue //如果解析消息失败，则发送错误事件
		}

		switch strings.TrimSpace(event.Type) {
		case "message:send":
			h.handleMessageSend(client, event.Data) //处理发送消息事件
		case "message:conversations":
			h.handleConversations(client, event.Data) //处理获取会话列表事件
		case "message:history":
			h.handleHistory(client, event.Data) //处理获取历史消息事件
		default:
			_ = client.WriteJSON(realtime.MessageEvent{Type: "message:error", Data: gin.H{"message": "unsupported event type"}})
		} //如果事件类型不支持，则发送错误事件
	}
	//==========清理==========
	//关闭停止ping通道
	close(stopPing)
}

// 处理发送消息事件
func (h *WSHandler) handleMessageSend(client *realtime.Client, raw json.RawMessage) {
	var req wsSendMessageData //解析发送消息请求
	if err := json.Unmarshal(raw, &req); err != nil {
		_ = client.WriteJSON(realtime.MessageEvent{Type: "message:error", Data: gin.H{"message": "invalid message payload"}}) //发送错误事件
		return
	}

	req.Content = strings.TrimSpace(req.Content)                           //去除消息内容前后空格
	if req.Content == "" || len([]rune(req.Content)) > maxMessageContent { //如果消息内容为空或长度大于最大长度，则发送错误事件
		_ = client.WriteJSON(realtime.MessageEvent{Type: "message:error", Data: gin.H{"client_msg_id": req.ClientMsgID, "message": "消息内容长度无效"}})
		return
	}

	msg, err := h.messageService.SendMessage(client.UserID, &services.SendMessageRequest{ //发送消息
		ToUserID: req.ToUserID,
		Content:  req.Content,
	})
	if err != nil {
		_ = client.WriteJSON(realtime.MessageEvent{Type: "message:error", Data: gin.H{"client_msg_id": req.ClientMsgID, "message": err.Error()}}) //发送错误事件
		return
	}

	_ = client.WriteJSON(realtime.MessageEvent{Type: "message:ack", Data: gin.H{"client_msg_id": req.ClientMsgID, "message": msg}}) //发送确认事件
}

// 处理获取会话列表事件
func (h *WSHandler) handleConversations(client *realtime.Client, raw json.RawMessage) {
	var req wsConversationsData //解析获取会话列表请求
	_ = json.Unmarshal(raw, &req)
	if req.Page < 1 {
		req.Page = 1 //如果页码小于1，则设置为1
	}
	if req.PageSize < 1 || req.PageSize > 50 {
		req.PageSize = 50 //如果页大小小于1或大于50，则设置为50
	}

	list, total, err := h.messageService.GetConversationList(client.UserID, req.Page, req.PageSize) //获取会话列表
	if err != nil {
		_ = client.WriteJSON(realtime.MessageEvent{Type: "message:error", Data: gin.H{"req_id": req.ReqID, "message": "获取会话失败"}}) //发送错误事件
		return
	}

	_ = client.WriteJSON(realtime.MessageEvent{Type: "message:conversations", Data: gin.H{"req_id": req.ReqID, "list": list, "total": total, "page": req.Page, "page_size": req.PageSize}}) //发送会话列表事件
}

// 处理获取历史消息事件
func (h *WSHandler) handleHistory(client *realtime.Client, raw json.RawMessage) {
	var req wsHistoryData //解析获取历史消息请求
	if err := json.Unmarshal(raw, &req); err != nil {
		_ = client.WriteJSON(realtime.MessageEvent{Type: "message:error", Data: gin.H{"message": "invalid history payload"}}) //发送错误事件
		return
	}
	if req.TargetUser == 0 {
		_ = client.WriteJSON(realtime.MessageEvent{Type: "message:error", Data: gin.H{"req_id": req.ReqID, "message": "target_user_id 无效"}}) //发送错误事件
		return
	}
	if req.Page < 1 {
		req.Page = 1 //如果页码小于1，则设置为1
	}
	if req.PageSize < 1 || req.PageSize > 100 {
		req.PageSize = 100 //如果页大小小于1或大于100，则设置为100
	}

	list, total, err := h.messageService.GetConversationMessages(client.UserID, req.TargetUser, req.Page, req.PageSize) //获取历史消息
	if err != nil {
		_ = client.WriteJSON(realtime.MessageEvent{Type: "message:error", Data: gin.H{"req_id": req.ReqID, "message": err.Error()}}) //发送错误事件
		return
	}

	_ = client.WriteJSON(realtime.MessageEvent{Type: "message:history", Data: gin.H{"req_id": req.ReqID, "target_user_id": req.TargetUser, "list": list, "total": total, "page": req.Page, "page_size": req.PageSize}}) //发送历史消息事件
}

// 心跳检测协程
func (h *WSHandler) keepAlive(client *realtime.Client, stop <-chan struct{}) {
	//创建心跳检测定时器
	ticker := time.NewTicker(wsPingInterval)
	defer ticker.Stop()
	for {
		select {
		case <-stop:
			// 外部触发关闭（客户端主动断开、业务关闭），直接退出心跳协程
			return
		case <-ticker.C:
			// ========== 1. 先判断token是否过期 ==========
			if time.Now().After(client.ExpiresAt) {
				// 下发业务事件：通知前端token过期
				_ = client.WriteJSON(realtime.MessageEvent{Type: "auth:expired", Data: gin.H{"message": "token expired"}})                                                    //发送令牌过期事件
				_ = client.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.ClosePolicyViolation, "token expired"), time.Now().Add(2*time.Second)) //发送关闭连接事件
				_ = client.Conn.Close()                                                                                                                                       //关闭连接
				return                                                                                                                                                        //如果连接关闭，则返回
			}
			// ========== 2. Token没过期，发送Ping心跳帧 ==========
			// 设置写超时，防止写阻塞卡死协程
			_ = client.Conn.SetWriteDeadline(time.Now().Add(wsWriteWait))
			// 发送websocket Ping控制帧，payload "ping"
			if err := client.WriteControl(websocket.PingMessage, []byte("ping"), time.Now().Add(wsWriteWait)); err != nil {
				// Ping发送失败：网络断了、连接已失效，关闭连接退出
				_ = client.Conn.Close()
				return
			}
		}
	}
}
