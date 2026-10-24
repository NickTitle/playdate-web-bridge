package simulator

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"net"
	"os"
	"sync"
)

// AudioReceiver continuously drains the native tap. Only the newest block is
// retained when playback stalls; no recording files or unbounded queues exist.
// Packets retain capture-clock timestamps for subsequent synchronization work.
type AudioReceiver struct {
	conn    *net.UnixConn
	packets chan []byte
	done    chan struct{}
	once    sync.Once
}

func OpenAudioReceiver(path string) (*AudioReceiver, error) {
	// Never unlink an existing socket: it may belong to a different session.
	conn, err := net.ListenUnixgram("unixgram", &net.UnixAddr{Name: path, Net: "unixgram"})
	if err != nil {
		return nil, err
	}
	if err = os.Chmod(path, 0600); err != nil {
		conn.Close()
		os.Remove(path)
		return nil, err
	}
	r := &AudioReceiver{conn: conn, packets: make(chan []byte, 1), done: make(chan struct{})}
	go func() {
		defer close(r.done)
		defer close(r.packets)
		buf := make([]byte, 65553)
		for {
			n, _, err := conn.ReadFromUnix(buf)
			if err != nil {
				return
			}
			if !validAudioPacket(buf[:n]) {
				continue
			}
			packet := bytes.Clone(buf[:n])
			select {
			case r.packets <- packet:
			default:
				select {
				case <-r.packets:
				default:
				}
				select {
				case r.packets <- packet:
				default:
				}
			}
		}
	}()
	return r, nil
}

func validAudioPacket(p []byte) bool {
	return len(p) > 16 && len(p) <= 65552 && string(p[:4]) == "SFPC" &&
		int(binary.LittleEndian.Uint32(p[4:8])) == len(p)-16 && (len(p)-16)%4 == 0 &&
		binary.LittleEndian.Uint64(p[8:16]) > 0
}
func (r *AudioReceiver) Packets() <-chan []byte { return r.packets }
func (r *AudioReceiver) Close() {
	r.once.Do(func() { r.conn.Close(); <-r.done; os.Remove(r.conn.LocalAddr().String()) })
}

// Explicit opt-in only; inherited capture settings remain stripped.
type AudioCapture struct{ Library, Socket string }

func (a *AudioCapture) environment() (map[string]string, error) {
	if a.Library == "" || a.Socket == "" {
		return nil, fmt.Errorf("audio library and socket required")
	}
	if _, err := os.Stat(a.Library); err != nil {
		return nil, err
	}
	info, err := os.Stat(a.Socket)
	if err != nil {
		return nil, err
	}
	if info.Mode()&os.ModeSocket == 0 {
		return nil, fmt.Errorf("audio receiver is not a socket")
	}
	return map[string]string{"LD_PRELOAD": a.Library, "PLAYDATE_BRIDGE_AUDIO_SOCKET": a.Socket, "SDL_AUDIODRIVER": "disk", "SDL_DISKAUDIOFILE": "/dev/null"}, nil
}
