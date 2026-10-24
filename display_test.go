package simulator

import (
	"bufio"
	"bytes"
	"context"
	"image"
	"image/png"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

func TestXImagePixels(t *testing.T) {
	encoded, err := encodeXImage([]byte{0, 0, 255, 0, 255, 0, 0, 0}, 2, 1)
	if err != nil {
		t.Fatal(err)
	}
	img, err := png.Decode(bytes.NewReader(encoded))
	if err != nil {
		t.Fatal(err)
	}
	r, g, b, a := img.At(0, 0).RGBA()
	if r != 65535 || g != 0 || b != 0 || a != 65535 {
		t.Fatal("red/alpha corrupted")
	}
	r, g, b, a = img.At(1, 0).RGBA()
	if r != 0 || g != 0 || b != 65535 || a != 65535 {
		t.Fatal("blue/alpha corrupted")
	}
	for _, size := range [][2]int{{0, 1}, {-1, 1}, {4097, 1}, {2, 2}} {
		if _, err := encodeXImage(make([]byte, 8), size[0], size[1]); err == nil {
			t.Fatal("accepted invalid image", size)
		}
	}
}

// Explicit opt-in keeps ordinary tests independent of an installed X server.
func TestPrivateXDisplay(t *testing.T) {
	if os.Getenv("PLAYDATE_BRIDGE_TEST_X11") != "1" {
		t.Skip("set PLAYDATE_BRIDGE_TEST_X11=1 for bounded private Xvfb check")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	cmd := exec.Command("Xvfb", "-displayfd", "3", "-screen", "0", "1024x768x24", "-nolisten", "tcp")
	cmd.ExtraFiles = []*os.File{writer}
	done, err := startOwned(ctx, cmd)
	writer.Close()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { cancel(); <-done }()
	line, err := bufio.NewReader(reader).ReadString('\n')
	if err != nil {
		t.Fatal(err)
	}
	d, err := OpenDisplay(":" + strings.TrimSpace(line))
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	frame, err := d.Capture(image.Rect(0, 0, 400, 240))
	if err != nil {
		t.Fatal(err)
	}
	img, err := png.Decode(bytes.NewReader(frame))
	if err != nil || img.Bounds().Dx() != 400 || img.Bounds().Dy() != 240 {
		t.Fatal("invalid live frame", err)
	}
	if err := d.Key(0xff52, true); err != nil {
		t.Fatal(err)
	}
	if err := d.Key(0xff52, false); err != nil {
		t.Fatal(err)
	}
	if _, err := d.Capture(image.Rect(-1, 0, 400, 240)); err == nil {
		t.Fatal("invalid crop accepted")
	}
}
