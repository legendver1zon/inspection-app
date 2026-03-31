package handlers

import (
	"encoding/json"
	"inspection-app/internal/logger"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

// --- WebSocket Hub для push-уведомлений о загрузке фото ---

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

// wsHub хранит подписки: inspectionID → набор соединений
var wsHub = struct {
	sync.RWMutex
	conns map[uint]map[*websocket.Conn]struct{}
}{conns: make(map[uint]map[*websocket.Conn]struct{})}

// WsUploadStatus — GET /inspections/:id/ws
// Устанавливает WebSocket-соединение для получения обновлений upload-status.
func WsUploadStatus(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Неверный ID"})
		return
	}
	inspectionID := uint(id)

	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		logger.Error("ws upgrade failed", "error", err)
		return
	}

	// Регистрируем соединение
	wsHub.Lock()
	if wsHub.conns[inspectionID] == nil {
		wsHub.conns[inspectionID] = make(map[*websocket.Conn]struct{})
	}
	wsHub.conns[inspectionID][conn] = struct{}{}
	wsHub.Unlock()

	// Читаем сообщения (нужно для обнаружения закрытия соединения)
	go func() {
		defer func() {
			wsHub.Lock()
			delete(wsHub.conns[inspectionID], conn)
			if len(wsHub.conns[inspectionID]) == 0 {
				delete(wsHub.conns, inspectionID)
			}
			wsHub.Unlock()
			conn.Close()
		}()

		conn.SetReadLimit(512)
		conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		conn.SetPongHandler(func(string) error {
			conn.SetReadDeadline(time.Now().Add(60 * time.Second))
			return nil
		})

		for {
			_, _, err := conn.ReadMessage()
			if err != nil {
				break
			}
		}
	}()

	// Ping каждые 30 секунд чтобы держать соединение
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			wsHub.RLock()
			_, exists := wsHub.conns[inspectionID][conn]
			wsHub.RUnlock()
			if !exists {
				return
			}
			if err := conn.WriteControl(websocket.PingMessage, nil, time.Now().Add(5*time.Second)); err != nil {
				return
			}
		}
	}()
}

// NotifyUploadStatus отправляет обновление статуса загрузки всем подписчикам inspection.
// Вызывается из worker после загрузки фото.
func NotifyUploadStatus(inspectionID uint, data map[string]interface{}) {
	wsHub.RLock()
	clients := wsHub.conns[inspectionID]
	if len(clients) == 0 {
		wsHub.RUnlock()
		return
	}
	// Копируем список чтобы не держать lock
	conns := make([]*websocket.Conn, 0, len(clients))
	for c := range clients {
		conns = append(conns, c)
	}
	wsHub.RUnlock()

	msg, err := json.Marshal(data)
	if err != nil {
		return
	}

	for _, conn := range conns {
		if err := conn.WriteMessage(websocket.TextMessage, msg); err != nil {
			// Удаляем битое соединение
			wsHub.Lock()
			delete(wsHub.conns[inspectionID], conn)
			wsHub.Unlock()
			conn.Close()
		}
	}
}
