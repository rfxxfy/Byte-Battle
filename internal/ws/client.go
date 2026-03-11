package ws

import (
	"time"

	"github.com/gorilla/websocket"
)

const (
	writeWait  = 10 * time.Second
	PongWait   = 60 * time.Second
	pingPeriod = (PongWait * 9) / 10
)

type Client struct {
	send chan []byte
	conn *websocket.Conn
}

func NewClient(conn *websocket.Conn) *Client {
	return &Client{
		conn: conn,
		send: make(chan []byte, 64),
	}
}

// Close signals WritePump to exit cleanly by closing the send channel.
func (c *Client) Close() { close(c.send) }

// WritePump pumps messages from the send channel to the WebSocket connection.
// Run in a separate goroutine; closes the connection when done.
func (c *Client) WritePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case msg, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, nil) //nolint:errcheck // connection is closing, error not actionable
				return
			}
			if err := c.conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				return
			}
		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
