package realtime

import (
	"encoding/json"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// MessageEvent 为消息实时推送事件。
type MessageEvent struct {
	Type string `json:"type"`
	Data any    `json:"data"`
}

// Client 表示一个在线 WebSocket 连接。
type Client struct {
	UserID    uint
	Conn      *websocket.Conn
	ExpiresAt time.Time
	mu        sync.Mutex
}

func (c *Client) WriteJSON(v any) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.Conn.WriteJSON(v)
}

func (c *Client) WriteMessage(messageType int, data []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.Conn.WriteMessage(messageType, data)
}

func (c *Client) WriteControl(messageType int, data []byte, deadline time.Time) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.Conn.WriteControl(messageType, data, deadline)
}

var (
	hubMu       sync.RWMutex                      // 互斥锁
	userSockets = map[uint]map[*Client]struct{}{} // 用户ID -> 客户端连接
)

// RegisterConn 注册用户 WebSocket 连接。
func RegisterConn(client *Client) {
	hubMu.Lock()
	defer hubMu.Unlock()
	if _, ok := userSockets[client.UserID]; !ok { // 如果用户没有在线连接，则注册用户
		userSockets[client.UserID] = map[*Client]struct{}{}
	}
	userSockets[client.UserID][client] = struct{}{} // 注册客户端连接 支持多端登录
}

// UnregisterConn 注销用户 WebSocket 连接。
func UnregisterConn(client *Client) {
	hubMu.Lock()
	defer hubMu.Unlock()
	if conns, ok := userSockets[client.UserID]; ok {
		delete(conns, client) // 注销客户端连接
		if len(conns) == 0 {  // 如果用户没有在线连接，则注销用户
			delete(userSockets, client.UserID) // 注销用户
		}
	}
}

// PushToUser 推送事件到用户所有在线连接。（多端）
func PushToUser(userID uint, event MessageEvent) {
	hubMu.RLock()
	conns := userSockets[userID]
	hubMu.RUnlock()
	if len(conns) == 0 {
		return
	}

	payload, _ := json.Marshal(event)
	for client := range conns {
		if client.ExpiresAt.Before(time.Now()) { // 如果令牌过期，则关闭连接
			_ = client.WriteJSON(MessageEvent{Type: "auth:expired", Data: map[string]any{"message": "token expired"}})                                                    // 发送令牌过期事件
			_ = client.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.ClosePolicyViolation, "token expired"), time.Now().Add(2*time.Second)) // 发送关闭连接事件
			_ = client.Conn.Close()                                                                                                                                       // 关闭连接
			UnregisterConn(client)                                                                                                                                        // 注销连接
			continue
		}
		if err := client.WriteMessage(websocket.TextMessage, payload); err != nil { //接收方：读取接收者接受的消息（写入消息 前端读取）
			_ = client.Conn.Close() // 关闭连接
			UnregisterConn(client)  // 注销连接
		}
	}
}
