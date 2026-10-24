package simulator

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

type fakeBackend struct {
	mu    sync.Mutex
	edges []Edge
	crank float64
}

func (b *fakeBackend) Ready() bool                           { return true }
func (b *fakeBackend) Frame(context.Context) ([]byte, error) { return []byte("test frame"), nil }
func (b *fakeBackend) Input(_ context.Context, e []Edge, c float64) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.edges = append(b.edges, e...)
	b.crank += c
	return nil
}
func TestTransportLeaseOriginAndExclusiveControl(t *testing.T) {
	b := &fakeBackend{}
	s := &Server{Backend: b, Origin: "https://example.test"}
	host := httptest.NewServer(s.Handler())
	defer host.Close()
	endpoint := "ws" + strings.TrimPrefix(host.URL, "http") + "/session"
	dial := func(origin string) (*websocket.Conn, *http.Response, error) {
		return websocket.DefaultDialer.Dial(endpoint, http.Header{"Origin": []string{origin}})
	}
	if c, r, e := dial("https://evil.test"); e == nil {
		c.Close()
		t.Fatal("foreign origin accepted")
	} else if r.StatusCode != 403 {
		t.Fatal(r.Status)
	}
	c, _, err := dial(s.Origin)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	if other, r, e := dial(s.Origin); e == nil {
		other.Close()
		t.Fatal("second controller accepted")
	} else if r.StatusCode != 409 {
		t.Fatal(r.Status)
	}
	c.WriteJSON(Snapshot{1, []string{"up", "b"}, 0})
	c.WriteJSON(Snapshot{2, []string{"up", "b"}, 45})
	// Read frames while the independent server lease expires without new snapshots.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		c.SetReadDeadline(deadline)
		kind, frame, e := c.ReadMessage()
		if e != nil {
			t.Fatal(e)
		}
		if kind != websocket.BinaryMessage || string(frame) != "test frame" {
			t.Fatal("frame transport")
		}
		b.mu.Lock()
		n := len(b.edges)
		crank := b.crank
		b.mu.Unlock()
		if n == 4 {
			if crank != 45 {
				t.Fatal(crank)
			}
			break
		}
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	want := []Edge{{"up", true}, {"b", true}, {"up", false}, {"b", false}}
	if len(b.edges) != len(want) {
		t.Fatal(b.edges)
	}
	for i := range want {
		if b.edges[i] != want[i] {
			t.Fatal(b.edges)
		}
	}
}
func TestDisconnectReleasesAndUnavailableHealth(t *testing.T) {
	s := &Server{Origin: "http://localhost"}
	r := httptest.NewRecorder()
	s.Handler().ServeHTTP(r, httptest.NewRequest("GET", "/healthz", nil))
	if r.Code != 503 {
		t.Fatal(r.Code)
	}
	b := &fakeBackend{}
	s.Backend = b
	host := httptest.NewServer(s.Handler())
	defer host.Close()
	c, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(host.URL, "http")+"/session", http.Header{"Origin": []string{s.Origin}})
	if err != nil {
		t.Fatal(err)
	}
	c.WriteJSON(Snapshot{1, []string{"left", "a"}, 0})
	c.SetReadDeadline(time.Now().Add(time.Second))
	if _, _, err = c.ReadMessage(); err != nil {
		t.Fatal(err)
	}
	c.Close()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		b.mu.Lock()
		n := len(b.edges)
		b.mu.Unlock()
		if n == 4 && !s.busy.Load() {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("disconnect did not release controls and exclusive lease")
}

func TestAudioMultiplexAndClosedSource(t *testing.T) {
	packets := make(chan []byte, 2)
	packet := make([]byte, 20)
	copy(packet, "SFPC")
	packet[4] = 4
	packet[8] = 1
	packets <- packet
	close(packets)
	s := &Server{Backend: &fakeBackend{}, Origin: "http://localhost", Audio: packets}
	host := httptest.NewServer(s.Handler())
	defer host.Close()
	c, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(host.URL, "http")+"/session", http.Header{"Origin": []string{s.Origin}})
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	c.SetReadDeadline(time.Now().Add(time.Second))
	gotAudio, gotFrame := false, false
	for !gotAudio || !gotFrame {
		kind, data, err := c.ReadMessage()
		if err != nil {
			t.Fatal(err)
		}
		if kind != websocket.BinaryMessage {
			t.Fatal(kind)
		}
		if string(data) == string(packet) {
			gotAudio = true
		} else if string(data) == "test frame" {
			gotFrame = true
		} else {
			t.Fatal("unexpected packet")
		}
	}
}
