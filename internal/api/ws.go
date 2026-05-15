package api

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/gorilla/websocket"

	apiv1 "github.com/k8s-ui/k8s-ui/pkg/api/v1"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin:     func(*http.Request) bool { return true }, // MVP; lock down later
}

// appEventsWS opens a multiplexed connection. The client sends one or more
// {"action":"subscribe","topic":"app:<name>:status"} messages; the server
// forwards every matching publish to the same socket as a WSMessage frame.
func (s *Server) appEventsWS(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	if name != "" {
		if _, ok := s.appByNameAuthorized(w, r, name); !ok {
			return
		}
	}
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		slog.Warn("ws upgrade", "err", err)
		return
	}
	defer conn.Close()

	sub := s.opts.Hub.Subscribe(64)
	defer sub.Close()

	if name != "" {
		sub.Listen("app:" + name + ":status")
		sub.Listen("app:" + name + ":sync")
	}

	conn.SetReadLimit(1 << 16)
	conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	doneRead := make(chan struct{})
	go func() {
		defer close(doneRead)
		for {
			_, msg, err := conn.ReadMessage()
			if err != nil {
				return
			}
			var cmd struct {
				Action string `json:"action"`
				Topic  string `json:"topic"`
			}
			if err := json.Unmarshal(msg, &cmd); err != nil {
				continue
			}
			switch cmd.Action {
			case "subscribe":
				sub.Listen(cmd.Topic)
			case "unsubscribe":
				sub.Unlisten(cmd.Topic)
			}
		}
	}()

	pingTicker := time.NewTicker(30 * time.Second)
	defer pingTicker.Stop()

	for {
		select {
		case msg, ok := <-sub.C():
			if !ok {
				return
			}
			if err := writeWS(conn, msg); err != nil {
				return
			}
		case <-pingTicker.C:
			conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		case <-doneRead:
			return
		}
	}
}

func writeWS(c *websocket.Conn, m apiv1.WSMessage) error {
	c.SetWriteDeadline(time.Now().Add(10 * time.Second))
	return c.WriteJSON(m)
}
