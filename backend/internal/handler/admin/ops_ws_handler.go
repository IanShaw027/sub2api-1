package admin

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // In production, we should check origin
	},
}

// QPSWSHandler handles realtime QPS push via WebSocket.
// GET /api/v1/admin/ops/ws/qps
func (h *OpsHandler) QPSWSHandler(c *gin.Context) {
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("[OpsWS] upgrade failed: %v", err)
		return
	}
	defer conn.Close()

	// Set pong handler
	conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	// Push QPS data every 2 seconds
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	// Heartbeat ping every 30 seconds
	pingTicker := time.NewTicker(30 * time.Second)
	defer pingTicker.Stop()

	ctx, cancel := context.WithCancel(c.Request.Context())
	defer cancel()

	for {
		select {
		case <-ticker.C:
			// Fetch 1m window stats for current QPS
			data, err := h.opsService.GetDashboardOverview(ctx, "5m")
			if err != nil {
				log.Printf("[OpsWS] get overview failed: %v", err)
				continue
			}

			payload := gin.H{
				"type":      "qps_update",
				"timestamp": time.Now().Format(time.RFC3339),
				"data": gin.H{
					"qps":           data.QPS.Current,
					"tps":           data.TPS.Current,
					"request_count": data.Errors.TotalCount + int64(data.QPS.Avg1h*60), // Rough estimate
				},
			}

			msg, _ := json.Marshal(payload)
			if err := conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				log.Printf("[OpsWS] write failed: %v", err)
				return
			}
		case <-pingTicker.C:
			if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				log.Printf("[OpsWS] ping failed: %v", err)
				return
			}
		case <-ctx.Done():
			return
		}
	}
}
