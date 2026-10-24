# Development

## Build and test

Go 1.22+ builds the server, a C compiler builds the Linux audio hook, and Node.js runs the browser component tests.

```sh
make build audio
make test
PLAYDATE_BRIDGE_TEST_X11=1 go test -race ./...
```

The last command includes a bounded private-Xvfb transport check. Tests cover input edges and crank gestures, isolated saves, process cleanup, image conversion, WebSocket/controller handling, and bounded audio playback.

## Compatibility

The backend is calibrated for the Linux Playdate SDK 3.1.2 default 479×706 simulator layout. Validation covers Starfield desktop-browser use and an SDK FlippyFish startup/rendering smoke test. Broader game, SDK-layout, and mobile-device compatibility is under evaluation.

Audio uses a Linux SDL capture hook. Its timestamps describe capture time; network conditions and browser scheduling also affect playback timing.

## CLI

| Flag | Purpose |
| --- | --- |
| `-sdk` | SDK directory; defaults to `PLAYDATE_SDK_PATH`. |
| `-cartridge` | Simulator-compatible `.pdx` game directory. |
| `-session` | Dedicated persistent session directory. |
| `-origin` | Exact URL origin opened by the remote browser. |
| `-listen` | Backend listener; defaults to `127.0.0.1:8080`. |
| `-audio-library` | Path to the compiled `audio_stream.so` hook. |
| `-xvfb` | Xvfb executable; defaults to `Xvfb` on PATH. |

[Architecture](how-it-works.md) · [Remote usage](usage.md) · [Quick start](../README.md)
