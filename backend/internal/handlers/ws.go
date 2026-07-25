package handlers

import (
	"encoding/json"
	"inspection-app/internal/logger"
	"inspection-app/internal/models"
	"inspection-app/internal/storage"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

// --- WebSocket Hub для push-уведомлений о загрузке фото ---

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		origin := r.Header.Get("Origin")
		// Пустой Origin (не из браузера) или совпадение хоста — разрешаем
		if origin == "" {
			return true
		}
		// Разрешаем если origin содержит тот же хост
		return r.Header.Get("Origin") == "http://"+r.Host ||
			r.Header.Get("Origin") == "https://"+r.Host
	},
}

// safeConn оборачивает websocket.Conn с мьютексом для потокобезопасной записи.
type safeConn struct {
	conn *websocket.Conn
	mu   sync.Mutex
}

func (sc *safeConn) writeJSON(msg []byte) error {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	return sc.conn.WriteMessage(websocket.TextMessage, msg)
}

func (sc *safeConn) writePing() error {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	return sc.conn.WriteControl(websocket.PingMessage, nil, time.Now().Add(5*time.Second))
}

// wsHub хранит подписки: inspectionID → набор соединений
var wsHub = struct {
	sync.RWMutex
	conns map[uint]map[*safeConn]struct{}
}{conns: make(map[uint]map[*safeConn]struct{})}

// WsUploadStatus — GET /inspections/:id/ws
// Устанавливает WebSocket-соединение для получения обновлений upload-status.
func WsUploadStatus(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Неверный ID"})
		return
	}
	inspectionID := uint(id)

	// Проверка доступа: владелец или admin
	userID := c.GetUint("userID")
	role := c.GetString("userRole")
	var inspection models.Inspection
	if err := storage.DB.First(&inspection, inspectionID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Осмотр не найден"})
		return
	}
	if role != "admin" && inspection.UserID != userID {
		c.JSON(http.StatusForbidden, gin.H{"error": "Доступ запрещён"})
		return
	}

	raw, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		logger.Error("ws upgrade failed", "error", err)
		return
	}
	conn := &safeConn{conn: raw}

	// Регистрируем соединение
	wsHub.Lock()
	if wsHub.conns[inspectionID] == nil {
		wsHub.conns[inspectionID] = make(map[*safeConn]struct{})
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
			conn.conn.Close()
		}()

		conn.conn.SetReadLimit(512)
		conn.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		conn.conn.SetPongHandler(func(string) error {
			conn.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
			return nil
		})

		for {
			_, _, err := conn.conn.ReadMessage()
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
			if err := conn.writePing(); err != nil {
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
	conns := make([]*safeConn, 0, len(clients))
	for c := range clients {
		conns = append(conns, c)
	}
	wsHub.RUnlock()

	msg, err := json.Marshal(data)
	if err != nil {
		return
	}

	for _, sc := range conns {
		if err := sc.writeJSON(msg); err != nil {
			wsHub.Lock()
			delete(wsHub.conns[inspectionID], sc)
			wsHub.Unlock()
			sc.conn.Close()
		}
	}
}
