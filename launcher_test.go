package simulator

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestEnvironmentExcludesCaptureAndInheritedDisplay(t *testing.T) {
	env := cleanEnvironment([]string{"PATH=/bin", "DISPLAY=:99", "LD_PRELOAD=test.so", "PLAYDATE_BRIDGE_AV_CAPTURE=1", "SDL_DISKAUDIOFILE=old.raw", "XDG_CONFIG_HOME=old"}, map[string]string{"DISPLAY": ":123", "XDG_CONFIG_HOME": "isolated", "SDL_AUDIODRIVER": "dummy"})
	joined := strings.Join(env, "\n")
	for _, forbidden := range []string{"test.so", "old", ":99", "PLAYDATE_BRIDGE_"} {
		if strings.Contains(joined, forbidden) {
			t.Fatal(joined)
		}
	}
	for _, required := range []string{"PATH=/bin", "DISPLAY=:123", "XDG_CONFIG_HOME=isolated", "SDL_AUDIODRIVER=dummy"} {
		if !strings.Contains(joined, required) {
			t.Fatal(joined)
		}
	}
}

func executable(t *testing.T, root, name, script string) string {
	t.Helper()
	path := filepath.Join(root, name)
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"+script), 0700); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLauncherIsolatedEnvironmentAndCancellation(t *testing.T) {
	root := t.TempDir()
	display := executable(t, root, "display", "echo 123 >&3\nexec /bin/sleep 20\n")
	game := executable(t, root, "game", "env > environment.tmp\nmv environment.tmp environment\nexec /bin/sleep 20\n")
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	connected := false
	l := Launcher{Paths: SessionPaths{Root: root, Executable: game, Cartridge: "production.pdx", SDK: root + "/sdk", Config: root + "/config", Data: root + "/data"}, Xvfb: display, Connect: func(ctx context.Context, address string) error {
		connected = address == ":123"
		for {
			if _, err := os.Stat(filepath.Join(root, "environment")); err == nil {
				break
			}
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(time.Millisecond):
			}
		}
		cancel()
		return ctx.Err()
	}}
	if err := l.Run(ctx); err == nil {
		t.Fatal("expected cancellation")
	}
	if !connected {
		t.Fatal("backend did not receive allocated display")
	}
	env, err := os.ReadFile(filepath.Join(root, "environment"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"DISPLAY=:123", "PLAYDATE_SDK_PATH=" + root + "/sdk", "SDL_AUDIODRIVER=dummy"} {
		if !strings.Contains(string(env), want) {
			t.Fatal(string(env))
		}
	}
}

func TestLauncherRejectsInvalidDisplay(t *testing.T) {
	root := t.TempDir()
	display := executable(t, root, "display", "echo invalid >&3\n")
	l := Launcher{Paths: SessionPaths{Executable: "/must/not/start"}, Xvfb: display, Connect: func(context.Context, string) error { t.Fatal("connected to invalid display"); return nil }}
	if err := l.Run(context.Background()); err == nil || !strings.Contains(err.Error(), "invalid Xvfb") {
		t.Fatal(err)
	}
}
