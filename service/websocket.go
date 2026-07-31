package service

import (
	"log"
	"net/http"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		origin := r.Header.Get("Origin")

		allowedOrigins := []string{
			"http://localhost:8080",
			"http://localhost:8000",
		}

		for _, allowed := range allowedOrigins {
			if origin == allowed {
				return true
			}
		}

		log.Printf("WebSocket连接被拒绝: Origin=%s", origin)
		return false
	},
}

type ProgressMessage struct {
	TaskID   string `json:"task_id"`
	Status   string `json:"status"`
	Progress int    `json:"progress"`
	Error    string `json:"error,omitempty"`
}

type WebSocketManager struct {
	clients map[string]map[*websocket.Conn]bool
	mu      sync.Mutex
}

var wsManager *WebSocketManager

func InitWebSocket() {
	wsManager = &WebSocketManager{
		clients: make(map[string]map[*websocket.Conn]bool),
	}
}

func (m *WebSocketManager) Register(taskID string, conn *websocket.Conn) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.clients[taskID] == nil {
		m.clients[taskID] = make(map[*websocket.Conn]bool)
	}
	m.clients[taskID][conn] = true
}

func (m *WebSocketManager) Unregister(taskID string, conn *websocket.Conn) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if clients, exists := m.clients[taskID]; exists {
		delete(clients, conn)
		conn.Close()
		if len(clients) == 0 {
			delete(m.clients, taskID)
		}
	}
}

func (m *WebSocketManager) BroadcastProgress(taskID, status string, progress int, errMsg string) {
	msg := ProgressMessage{
		TaskID:   taskID,
		Status:   status,
		Progress: progress,
		Error:    errMsg,
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	clients, exists := m.clients[taskID]
	if !exists {
		return
	}

	for conn := range clients {
		if err := conn.WriteJSON(msg); err != nil {
			log.Printf("WebSocket发送失败: %v", err)
			delete(clients, conn)
			conn.Close()
		}
	}

	if progress == 100 || status == "error" {
		for conn := range clients {
			conn.Close()
			delete(clients, conn)
		}
		delete(m.clients, taskID)
	}
}

func WebSocketHandler(c *gin.Context) {
	taskID := c.Query("task_id")
	if taskID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少 task_id 参数"})
		return
	}

	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("WebSocket升级失败: %v", err)
		return
	}
	defer wsManager.Unregister(taskID, conn)

	wsManager.Register(taskID, conn)

	for {
		_, _, err := conn.ReadMessage()
		if err != nil {
			break
		}
	}
}

func BroadcastProgress(taskID, status string, progress int, errMsg string) {
	if wsManager != nil {
		wsManager.BroadcastProgress(taskID, status, progress, errMsg)
	}
}
