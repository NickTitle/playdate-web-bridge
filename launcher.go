package simulator

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// Launch runs the display and production simulator for the lifetime of ctx.
// It is a foreground supervisor, not a daemon. Connect is called only after
// Xvfb has allocated a free display; it must return promptly on cancellation.
// Display/session directories must never be shared with automated game tests.
type Launcher struct {
	Audio   *AudioCapture
	Paths   SessionPaths
	Xvfb    string
	Output  io.Writer
	Connect func(context.Context, string) error
}

func cleanEnvironment(base []string, values map[string]string) []string {
	out := []string{}
	for _, entry := range base {
		key, _, _ := strings.Cut(entry, "=")
		if _, replace := values[key]; replace {
			continue
		}
		// Test recording hooks must not enter the interactive simulator.
		if strings.HasPrefix(key, "PLAYDATE_BRIDGE_") || strings.HasPrefix(key, "SDL_") || key == "LD_PRELOAD" || key == "DISPLAY" {
			continue
		}
		out = append(out, entry)
	}
	for key, value := range values {
		out = append(out, key+"="+value)
	}
	return out
}

// startOwned gives each child a process group and always reaps it. Cancellation
// kills the group, including descendants, so a failed launch cannot leave Xvfb.
func startOwned(ctx context.Context, cmd *exec.Cmd) (<-chan error, error) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	done := make(chan error, 1)
	go func() {
		stopped := make(chan struct{})
		go func() {
			select {
			case <-ctx.Done():
				syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
			case <-stopped:
			}
		}()
		err := cmd.Wait()
		// Reap any descendants even if the immediate child exited first.
		syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		close(stopped)
		done <- err
	}()
	return done, nil
}

func (l Launcher) Run(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	if l.Xvfb == "" || l.Paths.Executable == "" || l.Connect == nil {
		return fmt.Errorf("launcher requires display, simulator and backend connector")
	}
	reader, writer, err := os.Pipe()
	if err != nil {
		return err
	}
	defer reader.Close()
	display := exec.Command(l.Xvfb, "-displayfd", "3", "-screen", "0", "1024x768x24", "-nolisten", "tcp")
	display.ExtraFiles = []*os.File{writer}
	display.Stdout, display.Stderr = l.Output, l.Output
	display.Env = cleanEnvironment(os.Environ(), nil)
	displayDone, err := startOwned(ctx, display)
	writer.Close()
	if err != nil {
		return err
	}
	defer func() { cancel(); <-displayDone }()
	numbers := make(chan string, 1)
	go func() { line, _ := bufio.NewReader(reader).ReadString('\n'); numbers <- strings.TrimSpace(line) }()
	var number string
	timer := time.NewTimer(10 * time.Second)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return fmt.Errorf("display allocation timed out")
	case number = <-numbers:
	}
	n, err := strconv.Atoi(number)
	if err != nil || n < 0 {
		return fmt.Errorf("invalid Xvfb display %q", number)
	}
	address := ":" + number
	game := exec.Command(l.Paths.Executable, l.Paths.Cartridge)
	game.Dir = l.Paths.Root
	environment := map[string]string{
		"DISPLAY": address, "PLAYDATE_SDK_PATH": l.Paths.SDK,
		"XDG_CONFIG_HOME": l.Paths.Config, "XDG_DATA_HOME": l.Paths.Data,
		// Audio is disabled unless the optional capture library is supplied.
		"SDL_AUDIODRIVER": "dummy",
	}
	if l.Audio != nil {
		audio, err := l.Audio.environment()
		if err != nil {
			return err
		}
		for key, value := range audio {
			environment[key] = value
		}
	}
	game.Env = cleanEnvironment(os.Environ(), environment)
	game.Stdout, game.Stderr = l.Output, l.Output
	gameDone, err := startOwned(ctx, game)
	if err != nil {
		return err
	}
	defer func() { cancel(); <-gameDone }()
	connected := make(chan error, 1)
	go func() { connected <- l.Connect(ctx, address) }()
	// Wait channels are consumed here, so replace them before deferred cleanup.
	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-connected:
		return err
	case err := <-gameDone:
		gameDone = closedResult()
		return fmt.Errorf("simulator exited: %v", err)
	case err := <-displayDone:
		displayDone = closedResult()
		return fmt.Errorf("display exited: %v", err)
	}
}

func closedResult() <-chan error { ch := make(chan error); close(ch); return ch }
