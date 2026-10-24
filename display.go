package simulator

import (
	"bytes"
	"fmt"
	"image"
	"image/png"

	"github.com/jezek/xgb"
	"github.com/jezek/xgb/xproto"
	"github.com/jezek/xgb/xtest"
)

// Display is confined to the launcher's private X server. It must never attach
// to the desktop or the automated test display. Calls are owned by one backend.
// This is a transport primitive, not a readiness claim or complete Backend.
type Display struct {
	conn   *xgb.Conn
	screen *xproto.ScreenInfo
	keys   map[uint32]byte
}

func OpenDisplay(address string) (*Display, error) {
	c, err := xgb.NewConnDisplay(address)
	if err != nil {
		return nil, err
	}
	fail := func(err error) (*Display, error) { c.Close(); return nil, err }
	if err = xtest.Init(c); err != nil {
		return fail(err)
	}
	setup := xproto.Setup(c)
	screen := setup.DefaultScreen(c)
	// Launcher requests a 24-bit TrueColor screen. Reject unfamiliar layouts
	// rather than silently producing corrupted browser images.
	supported := false
	for _, f := range setup.PixmapFormats {
		if f.Depth == 24 && f.BitsPerPixel == 32 && f.ScanlinePad == 32 {
			supported = true
		}
	}
	if screen.RootDepth != 24 || setup.ImageByteOrder != xproto.ImageOrderLSBFirst || !supported {
		return fail(fmt.Errorf("unsupported X image format"))
	}
	visualOK := false
	for _, depth := range screen.AllowedDepths {
		for _, v := range depth.Visuals {
			if v.VisualId == screen.RootVisual && v.RedMask == 0xff0000 && v.GreenMask == 0xff00 && v.BlueMask == 0xff {
				visualOK = true
			}
		}
	}
	if !visualOK {
		return fail(fmt.Errorf("unsupported X visual"))
	}
	mapping, err := xproto.GetKeyboardMapping(c, setup.MinKeycode, byte(setup.MaxKeycode-setup.MinKeycode+1)).Reply()
	if err != nil {
		return fail(err)
	}
	keys := map[uint32]byte{}
	for n, sym := range mapping.Keysyms {
		if sym != 0 {
			if _, ok := keys[uint32(sym)]; !ok {
				keys[uint32(sym)] = byte(setup.MinKeycode) + byte(n/int(mapping.KeysymsPerKeycode))
			}
		}
	}
	return &Display{c, screen, keys}, nil
}
func (d *Display) Close() { d.conn.Close() }
func (d *Display) Key(keysym uint32, down bool) error {
	key, ok := d.keys[keysym]
	if !ok {
		return fmt.Errorf("unmapped keysym %x", keysym)
	}
	kind := byte(xproto.KeyRelease)
	if down {
		kind = xproto.KeyPress
	}
	return xtest.FakeInputChecked(d.conn, kind, key, 0, d.screen.Root, 0, 0, 0).Check()
}

// Capture returns real X server pixels. Crop must be calibrated against the
// actual simulator window before this can be used as the 400x240 game stream.
func (d *Display) Capture(crop image.Rectangle) ([]byte, error) {
	bounds := image.Rect(0, 0, int(d.screen.WidthInPixels), int(d.screen.HeightInPixels))
	if crop.Empty() || !crop.In(bounds) {
		return nil, fmt.Errorf("capture outside display")
	}
	reply, err := xproto.GetImage(d.conn, xproto.ImageFormatZPixmap, xproto.Drawable(d.screen.Root), int16(crop.Min.X), int16(crop.Min.Y), uint16(crop.Dx()), uint16(crop.Dy()), 0xffffffff).Reply()
	if err != nil {
		return nil, err
	}
	return encodeXImage(reply.Data, crop.Dx(), crop.Dy())
}
func encodeXImage(raw []byte, w, h int) ([]byte, error) {
	if w <= 0 || h <= 0 || w > 4096 || h > 4096 || len(raw) != w*h*4 {
		return nil, fmt.Errorf("invalid X image dimensions or data")
	}
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for n := 0; n < len(raw); n += 4 {
		img.Pix[n], img.Pix[n+1], img.Pix[n+2], img.Pix[n+3] = raw[n+2], raw[n+1], raw[n], 255
	}
	var out bytes.Buffer
	err := png.Encode(&out, img)
	return out.Bytes(), err
}

// Pointer sends bounded absolute pointer input only to this private display.
func (d *Display) Pointer(x, y int) error {
	if x < 0 || y < 0 || x >= int(d.screen.WidthInPixels) || y >= int(d.screen.HeightInPixels) {
		return fmt.Errorf("pointer outside display")
	}
	return xtest.FakeInputChecked(d.conn, xproto.MotionNotify, 0, 0, d.screen.Root, int16(x), int16(y), 0).Check()
}
func (d *Display) Click(x, y int) error {
	if err := d.Pointer(x, y); err != nil {
		return err
	}
	if err := xtest.FakeInputChecked(d.conn, xproto.ButtonPress, 1, 0, d.screen.Root, 0, 0, 0).Check(); err != nil {
		return err
	}
	return xtest.FakeInputChecked(d.conn, xproto.ButtonRelease, 1, 0, d.screen.Root, 0, 0, 0).Check()
}

// Window locates an exact SDK top-level window and returns its root geometry.
// This avoids assuming that the SDK always opens at a fixed desktop position.
func (d *Display) Window(title string) (xproto.Window, image.Rectangle, error) {
	tree, err := xproto.QueryTree(d.conn, d.screen.Root).Reply()
	if err != nil {
		return 0, image.Rectangle{}, err
	}
	for _, w := range tree.Children {
		name, err := xproto.GetProperty(d.conn, false, w, xproto.AtomWmName, xproto.GetPropertyTypeAny, 0, 1024).Reply()
		if err != nil || string(name.Value) != title {
			continue
		}
		attr, err := xproto.GetWindowAttributes(d.conn, w).Reply()
		if err != nil || attr.MapState != xproto.MapStateViewable {
			continue
		}
		g, err := xproto.GetGeometry(d.conn, xproto.Drawable(w)).Reply()
		if err != nil {
			return 0, image.Rectangle{}, err
		}
		return w, image.Rect(int(g.X), int(g.Y), int(g.X)+int(g.Width), int(g.Y)+int(g.Height)), nil
	}
	return 0, image.Rectangle{}, fmt.Errorf("SDK window %q not found", title)
}
func (d *Display) Focus(w xproto.Window) error {
	return xproto.SetInputFocusChecked(d.conn, xproto.InputFocusPointerRoot, w, xproto.TimeCurrentTime).Check()
}

// MouseButton allows a bounded crank drag without focusing a text-entry widget.
func (d *Display) MouseButton(down bool) error {
	kind := byte(xproto.ButtonRelease)
	if down {
		kind = xproto.ButtonPress
	}
	return xtest.FakeInputChecked(d.conn, kind, 1, 0, d.screen.Root, 0, 0, 0).Check()
}
