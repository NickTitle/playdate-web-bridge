package main

import (
	"context"
	"flag"
	simulator "github.com/NickTitle/playdate-web-bridge"
	"log"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"
)

func main() {
	listen := flag.String("listen", "127.0.0.1:8080", "HTTP listen address")
	origin := flag.String("origin", "", "exact browser origin (required)")
	session := flag.String("session", "", "absolute isolated persistent session directory (required)")
	sdk := flag.String("sdk", os.Getenv("PLAYDATE_SDK_PATH"), "installed SDK")
	cartridge := flag.String("cartridge", "", "path to a simulator-compatible .pdx directory (required)")
	audioLibrary := flag.String("audio-library", "", "optional bounded PCM capture library (experimental playback)")
	xvfb := flag.String("xvfb", "Xvfb", "Xvfb executable")
	flag.Parse()
	if *origin == "" || *session == "" || *sdk == "" || *cartridge == "" {
		log.Fatal("origin, session, sdk (or PLAYDATE_SDK_PATH), and cartridge required")
	}
	for _, p := range []*string{session, sdk, cartridge} {
		absolute, err := filepath.Abs(*p)
		if err != nil {
			log.Fatal(err)
		}
		*p = absolute
	}
	if *audioLibrary != "" {
		absolute, err := filepath.Abs(*audioLibrary)
		if err != nil {
			log.Fatal(err)
		}
		*audioLibrary = absolute
	}
	xvfbPath, err := exec.LookPath(*xvfb)
	if err != nil {
		log.Fatal(err)
	}
	paths, err := simulator.PrepareSession(*session, *sdk, *cartridge)
	if err != nil {
		log.Fatal(err)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	var audio *simulator.AudioReceiver
	var capture *simulator.AudioCapture
	if *audioLibrary != "" {
		socket := filepath.Join(*session, "audio.sock")
		audio, err = simulator.OpenAudioReceiver(socket)
		if err != nil {
			log.Fatal(err)
		}
		defer audio.Close()
		capture = &simulator.AudioCapture{Library: *audioLibrary, Socket: socket}
	}
	launcher := simulator.Launcher{Paths: paths, Audio: capture, Xvfb: xvfbPath, Output: os.Stderr, Connect: func(ctx context.Context, address string) error {
		startup, cancel := context.WithTimeout(ctx, 15*time.Second)
		backend, err := simulator.ConnectSDK(startup, address)
		cancel()
		if err != nil {
			return err
		}
		defer backend.Close()
		transport := &simulator.Server{Backend: backend, Origin: *origin}
		if audio != nil {
			transport.Audio = audio.Packets()
		}
		handler := transport.Handler()
		server := &http.Server{Addr: *listen, Handler: handler, ReadHeaderTimeout: 5 * time.Second, IdleTimeout: 30 * time.Second}
		done := make(chan struct{})
		go func() {
			select {
			case <-ctx.Done():
				server.Close()
			case <-done:
			}
		}()
		err = server.ListenAndServe()
		close(done)
		if err == http.ErrServerClosed {
			return nil
		}
		return err
	}}
	if err = launcher.Run(ctx); err != nil && ctx.Err() == nil {
		log.Fatal(err)
	}
}
