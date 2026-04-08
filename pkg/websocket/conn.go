package websocket

import (
	"encoding/json"
	"log/slog"
	"time"

	"github.com/gorilla/websocket"
)

// ReadPump reads messages from the WebSocket connection.
// Must be called in a dedicated goroutine.
func (c *Conn) ReadPump(onMessage func(msg *WsMessage), onDisconnect func()) {
	defer func() {
		c.hub.Unregister(c.ID)
		if onDisconnect != nil {
			onDisconnect()
		}
	}()

	c.ws.SetReadLimit(c.hub.maxMsgSize)
	c.ws.SetReadDeadline(time.Now().Add(c.hub.pongWait))
	c.ws.SetPongHandler(func(string) error {
		c.lastPing.Store(time.Now().UnixMilli())
		c.ws.SetReadDeadline(time.Now().Add(c.hub.pongWait))
		return nil
	})

	for {
		_, message, err := c.ws.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
				slog.Warn("ws read error", "conn_id", c.ID, "error", err)
			}
			return
		}

		var msg WsMessage
		if err := json.Unmarshal(message, &msg); err != nil {
			slog.Warn("ws invalid message", "conn_id", c.ID, "error", err)
			continue
		}

		onMessage(&msg)
	}
}

// WritePump writes messages from the send channel to the WebSocket connection.
// Must be called in a dedicated goroutine.
func (c *Conn) WritePump() {
	ticker := time.NewTicker(c.hub.pingPeriod)
	defer func() {
		ticker.Stop()
		c.ws.Close()
	}()

	for {
		select {
		case payload, ok := <-c.send:
			c.ws.SetWriteDeadline(time.Now().Add(c.hub.writeWait))
			if !ok {
				c.ws.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := c.ws.WriteMessage(websocket.TextMessage, payload); err != nil {
				return
			}

		case <-ticker.C:
			c.ws.SetWriteDeadline(time.Now().Add(c.hub.writeWait))
			if err := c.ws.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
