package simulator

import (
	"encoding/binary"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestAudioReceiver(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audio.sock")
	r, err := OpenAudioReceiver(path)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	if _, err := OpenAudioReceiver(path); err == nil {
		t.Fatal("replaced active socket")
	}
	c, err := net.Dial("unixgram", path)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	p := make([]byte, 20)
	copy(p, "SFPC")
	binary.LittleEndian.PutUint32(p[4:], 4)
	binary.LittleEndian.PutUint64(p[8:], 1)
	for _, bad := range [][]byte{nil, p[:15], append([]byte("WRNG"), p[4:]...), append(p, 0)} {
		if validAudioPacket(bad) {
			t.Fatal("accepted invalid packet")
		}
	}
	for i := 1; i <= 1000; i++ {
		binary.LittleEndian.PutUint64(p[8:], uint64(i))
		if _, err = c.Write(p); err != nil {
			t.Fatal(err)
		}
	}
	deadline := time.After(time.Second)
	for {
		select {
		case got := <-r.Packets():
			if binary.LittleEndian.Uint64(got[8:]) == 1000 {
				r.Close()
				if _, err := os.Stat(path); !os.IsNotExist(err) {
					t.Fatal("socket remains")
				}
				return
			}
		case <-deadline:
			t.Fatal("did not drain to newest packet")
		}
	}
}
