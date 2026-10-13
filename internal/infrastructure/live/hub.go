package live

import (
	"context"
	"sync"
	"time"

	"github.com/coder/websocket"
)

// Hub, canlı izleyicilere anlık görüntü dağıtır. Tek goroutine yazar;
// yavaş istemci atılır (bir kişiye takılıp herkesi bekletmez).
type Hub struct {
	mu        sync.Mutex
	clients   map[*client]struct{}
	broadcast chan []byte
}

type client struct {
	conn *websocket.Conn
	send chan []byte
}

func NewHub() *Hub {
	return &Hub{clients: make(map[*client]struct{}), broadcast: make(chan []byte, 16)}
}

// Run, hub döngüsüdür. stop kapanınca istemcileri kapatıp döner.
func (h *Hub) Run(stop <-chan struct{}) {
	for {
		select {
		case msg := <-h.broadcast:
			h.mu.Lock()
			for c := range h.clients {
				select {
				case c.send <- msg:
				default:
					close(c.send) // tıkanan istemci: düşür
					delete(h.clients, c)
				}
			}
			h.mu.Unlock()
		case <-stop:
			h.mu.Lock()
			for c := range h.clients {
				c.conn.Close(websocket.StatusGoingAway, "")
			}
			h.mu.Unlock()
			return
		}
	}
}

// Push, herkese gönderir; hub doluysa düşürür (kayıt yolunu bekletmez).
func (h *Hub) Push(msg []byte) {
	select {
	case h.broadcast <- msg:
	default:
	}
}

// Count, testler + demo için izleyici sayısıdır.
func (h *Hub) Count() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.clients)
}

// Attach, yükseltilmiş bağlantıyı hub'a bağlar. snapshot ilk mesajdır,
// ardından interval'de bir + Push'lar gelir. ctx bitince (istemci gidince)
// kaydı siler.
func (h *Hub) Attach(ctx context.Context, conn *websocket.Conn, snapshot func() []byte, interval time.Duration) {
	c := &client{conn: conn, send: make(chan []byte, 8)}
	h.mu.Lock()
	h.clients[c] = struct{}{}
	h.mu.Unlock()
	defer func() {
		h.mu.Lock()
		delete(h.clients, c)
		h.mu.Unlock()
		conn.Close(websocket.StatusNormalClosure, "")
	}()

	if first := snapshot(); first != nil {
		if err := conn.Write(ctx, websocket.MessageText, first); err != nil {
			return
		}
	}
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-c.send:
			if !ok {
				return
			}
			if err := conn.Write(ctx, websocket.MessageText, msg); err != nil {
				return
			}
		case <-t.C:
			if snap := snapshot(); snap != nil {
				if err := conn.Write(ctx, websocket.MessageText, snap); err != nil {
					return
				}
			}
		}
	}
}
