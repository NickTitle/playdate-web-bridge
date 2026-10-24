package simulator

import (
	"context"
	"fmt"
	"image"
	"math"
	"sync"
	"time"
)

// SDKBackend serializes capture and input on one isolated SDK display.
// Layout is intentionally pinned to the installed SDK's default 1x window.
type SDKBackend struct {
	mu      sync.Mutex
	display *Display
	bounds  image.Rectangle
	angle   float64
}

func ConnectSDK(ctx context.Context, address string) (*SDKBackend, error) {
	d, err := OpenDisplay(address)
	if err != nil {
		return nil, err
	}
	success := false
	defer func() {
		if !success {
			d.Close()
		}
	}()
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
			if _, dialog, e := d.Window("Playdate Developer eList"); e == nil {
				if e = d.Click(dialog.Min.X+226, dialog.Min.Y+169); e != nil {
					return nil, e
				}
				continue
			}
			w, bounds, e := d.Window("Playdate Simulator")
			if e != nil {
				continue
			}
			if bounds.Dx() != 479 || bounds.Dy() != 706 {
				return nil, fmt.Errorf("unsupported simulator layout: %v", bounds)
			}
			if e = d.Focus(w); e != nil {
				return nil, e
			}
			if e = d.Click(bounds.Min.X+218, bounds.Min.Y+163); e != nil {
				return nil, e
			}
			// Fresh isolated config defaults to docked. Dragging the dial below
			// also undocks it, avoiding a checkbox toggle on resumed sessions.
			success = true
			return &SDKBackend{display: d, bounds: bounds}, nil
		}
	}
}
func (b *SDKBackend) Ready() bool { b.mu.Lock(); defer b.mu.Unlock(); return b.display != nil }
func (b *SDKBackend) Close() {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.display != nil {
		for _, key := range sdkKeys {
			_ = b.display.Key(key, false)
		}
		_ = b.display.MouseButton(false)
		b.display.Close()
		b.display = nil
	}
}

var sdkKeys = map[string]uint32{"up": 0xff52, "down": 0xff54, "left": 0xff51, "right": 0xff53, "a": 's', "b": 'a'}

func (b *SDKBackend) Frame(ctx context.Context) ([]byte, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if b.display == nil {
		return nil, fmt.Errorf("SDK closed")
	}
	return b.display.Capture(image.Rect(b.bounds.Min.X+18, b.bounds.Min.Y+43, b.bounds.Min.X+418, b.bounds.Min.Y+283))
}
func (b *SDKBackend) Input(ctx context.Context, edges []Edge, crank float64) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}
	if b.display == nil {
		return fmt.Errorf("SDK closed")
	}
	if math.IsNaN(crank) || math.IsInf(crank, 0) || math.Abs(crank) > 720 {
		return fmt.Errorf("invalid crank")
	}
	for _, edge := range edges {
		if edge.Button == "menu" {
			if edge.Down {
				if err := b.display.Pointer(b.bounds.Min.X+453, b.bounds.Min.Y+94); err != nil {
					return err
				}
				if err := b.display.MouseButton(true); err != nil {
					return err
				}
				// A zero-duration click is missed by the SDK's input polling.
				time.Sleep(100 * time.Millisecond)
				if err := b.display.MouseButton(false); err != nil {
					return err
				}
			}
			continue
		}
		key, ok := sdkKeys[edge.Button]
		if !ok {
			return fmt.Errorf("invalid button")
		}
		if err := b.display.Key(key, edge.Down); err != nil {
			return err
		}
	}
	if crank == 0 {
		return nil
	}
	// Follow the circle in <=15-degree steps to preserve direction across zero.
	point := func(angle float64) error {
		rad := angle * math.Pi / 180
		return b.display.Pointer(b.bounds.Min.X+349+int(math.Round(50*math.Sin(rad))), b.bounds.Min.Y+616-int(math.Round(50*math.Cos(rad))))
	}
	if err := point(b.angle); err != nil {
		return err
	}
	if err := b.display.MouseButton(true); err != nil {
		return err
	}
	defer b.display.MouseButton(false)
	steps := int(math.Ceil(math.Abs(crank) / 15))
	for n := 0; n < steps; n++ {
		b.angle = math.Mod(b.angle+crank/float64(steps)+360, 360)
		if err := point(b.angle); err != nil {
			return err
		}
	}
	return nil
}
