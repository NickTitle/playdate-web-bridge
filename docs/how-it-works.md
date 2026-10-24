# How does it work?

The bridge runs Panic's official simulator on the Linux host and acts as a remote screen and controller.

## Video and controls

1. **Xvfb** provides an invisible desktop, where the simulator opens your `.pdx` game.
2. **X11** captures its 400×240 game area. The Go server sends PNG frames to the browser over WebSockets, targeting 30 frames per second.
3. Browser buttons become keyboard events. The virtual crank drags the simulator's crank widget using simulated mouse movements. Menu input clicks the simulator's menu control.

Screen capture and widget positions are calibrated to the SDK's default layout, so SDK layout changes can require coordinate updates.

## Audio

A small C library loads through Linux's `LD_PRELOAD`. With SDL's disk-audio backend selected, it intercepts outgoing PCM audio and forwards timestamped blocks through a local Unix socket to the Go server.

The server sends those blocks over WebSockets. Browser Web Audio schedules playback, bounds the queue, and drops stale packets after stalls. The audio boost slider adjusts playback gain.

## Sessions

Each bridge session has its own simulator settings and saved games. The supervisor owns the simulator and Xvfb processes and cleans them up together. One browser controls a session at a time; timeouts and disconnects release held inputs.

[Usage](usage.md) · [Quick start](../README.md)
