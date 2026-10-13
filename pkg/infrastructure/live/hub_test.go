package live

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/coder/websocket"
)

func TestPush(t *testing.T) {
	hub := NewHub()
	stop := make(chan struct{})
	defer close(stop)
	go hub.Run(stop)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		hub.Attach(r.Context(), conn, func() []byte { return []byte(`{"n":0}`) }, 50*time.Millisecond)
	}))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	url := "ws" + srv.URL[len("http"):]
	conn, _, err := websocket.Dial(ctx, url, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close(websocket.StatusNormalClosure, "")

	read := func() string {
		_, b, err := conn.Read(ctx)
		if err != nil {
			t.Fatalf("read: %v", err)
		}
		return string(b)
	}
	if got := read(); got != `{"n":0}` {
		t.Fatalf("ilk mesaj = %q", got)
	}
	if hub.Count() != 1 {
		t.Fatalf("izleyici = %d, istendi 1", hub.Count())
	}
	hub.Push([]byte(`{"n":1}`))
	if got := read(); got != `{"n":1}` {
		t.Fatalf("push mesajı = %q", got)
	}
	// Dolu hub kaydı düşürmez: taşırma push'u kayıt yolunu bekletmez.
	done := make(chan struct{})
	go func() { hub.Push([]byte("x")); close(done) }()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Push takıldı")
	}
}
