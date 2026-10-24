package simulator

import (
	"context"
	"embed"
	"encoding/json"
	"io/fs"
	"net/http"
	"net/url"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
)

// Backend must bound buffering and return promptly when ctx is cancelled.
// Frame returns the latest encoded PNG from the actual simulator, never a mock.
// Input is called serially, including releases on timeout and disconnect.
type Backend interface {
	Frame(context.Context) ([]byte, error)
	Input(context.Context, []Edge, float64) error
	Ready() bool
}

//go:embed web/*
var assets embed.FS

type Server struct {
	Backend Backend
	Audio   <-chan []byte
	// Origin is the exact externally reachable origin, including scheme and port.
	// Set explicitly behind the Tailscale HTTPS proxy; never trust forwarded headers.
	Origin string
	busy   atomic.Bool
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	web, _ := fs.Sub(assets, "web")
	mux.Handle("/", http.FileServer(http.FS(web)))
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		ready := s.Backend != nil && s.Backend.Ready()
		if !ready {
			w.WriteHeader(http.StatusServiceUnavailable)
		}
		json.NewEncoder(w).Encode(map[string]any{"simulator_ready": ready, "controller_connected": s.busy.Load(), "audio_available": s.Audio != nil})
	})
	mux.HandleFunc("/session", s.session)
	return mux
}
func (s *Server) session(w http.ResponseWriter, r *http.Request) {
	origin, err := url.Parse(s.Origin)
	if err != nil || origin.Host == "" || (origin.Scheme != "http" && origin.Scheme != "https") || r.Header.Get("Origin") != s.Origin {
		http.Error(w, "origin rejected", http.StatusForbidden)
		return
	}
	if s.Backend == nil || !s.Backend.Ready() {
		http.Error(w, "simulator unavailable", 503)
		return
	}
	if !s.busy.CompareAndSwap(false, true) {
		http.Error(w, "simulator already controlled", 409)
		return
	}
	defer s.busy.Store(false)
	upgrader := websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return r.Header.Get("Origin") == s.Origin }}
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close()
	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()
	var input Input
	defer func() {
		releaseCtx, done := context.WithTimeout(context.Background(), time.Second)
		defer done()
		s.Backend.Input(releaseCtx, input.Release(), 0)
	}()
	conn.SetReadLimit(2048)
	snapshots := make(chan Snapshot, 1)
	go func() {
		defer cancel()
		for {
			var snapshot Snapshot
			if conn.ReadJSON(&snapshot) != nil {
				return
			}
			select {
			case snapshots <- snapshot:
			case <-ctx.Done():
				return
			}
		}
	}()
	audio := s.Audio
	ticker := time.NewTicker(time.Second / 30)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case packet, ok := <-audio:
			if !ok {
				audio = nil
				continue
			}
			if !validAudioPacket(packet) {
				continue
			}
			conn.SetWriteDeadline(time.Now().Add(500 * time.Millisecond))
			if conn.WriteMessage(websocket.BinaryMessage, packet) != nil {
				return
			}
		case snapshot := <-snapshots:
			edges, crank, err := input.Apply(snapshot, time.Now())
			if err != nil {
				return
			}
			if s.Backend.Input(ctx, edges, crank) != nil {
				return
			}
		case now := <-ticker.C:
			if edges := input.Expire(now); len(edges) > 0 {
				if s.Backend.Input(ctx, edges, 0) != nil {
					return
				}
			}
			frame, err := s.Backend.Frame(ctx)
			if err != nil {
				return
			}
			if len(frame) == 0 {
				continue
			}
			conn.SetWriteDeadline(time.Now().Add(500 * time.Millisecond))
			if conn.WriteMessage(websocket.BinaryMessage, frame) != nil {
				return
			}
		}
	}
}
