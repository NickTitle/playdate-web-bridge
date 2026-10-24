# Playdate Web Bridge

Run a Playdate game in the official Linux simulator and control it from a browser, with touch buttons, a virtual crank, live 400×240 frames, and optional game audio.

This is a self-hosted bridge, not a Playdate emulator or a browser port of a game. You supply your own [Playdate SDK](https://play.date/dev/) and a simulator-compatible `.pdx` cartridge. Neither the SDK nor games are included.

Originally built to remotely test [Starfield for Playdate](https://github.com/NickTitle/starfield-playdate). This extraction removes Starfield assets, private host configuration, and its privileged deployment helper. It is an experimental Linux/SDK-specific developer tool, not a broadly validated emulator service.

## Requirements

- Linux with X11 libraries and Xvfb on PATH.
- Official Linux Playdate SDK. The backend is calibrated to SDK 3.1.2's default 479×706 simulator window; different versions/layouts may need adjustment.
- A simulator-compatible `.pdx` directory (device-only C binaries cannot run in the desktop simulator).
- Go 1.22 or newer; a C compiler for optional audio; Node.js for JavaScript tests.

## Build and run

```sh
make build
export PLAYDATE_SDK_PATH=/path/to/PlaydateSDK
./build/playdate-web-bridge \
  -cartridge /path/to/MyGame.pdx \
  -session /path/to/new-session \
  -origin http://localhost:8080
```

Open **http://localhost:8080**, then tap **Connect**. Use the D-pad, A/B, Menu, and virtual crank; their game actions depend on the loaded cartridge. One browser controls the shared session at a time. Disconnects and expired inputs release held controls.

The session directory must initially not exist or be an existing bridge-marked directory. Its parent must exist. Simulator settings and saves are isolated from the SDK installation and persist across reconnects. Each session belongs to its cartridge; use a different directory for another game. Do not point it at important existing data. The SDK path, cartridge and session may be supplied as relative or absolute CLI paths.

## Optional audio

```sh
make audio
./build/playdate-web-bridge \
  -sdk /path/to/PlaydateSDK \
  -cartridge /path/to/MyGame.pdx \
  -session /path/to/new-session \
  -origin http://localhost:8080 \
  -audio-library "$PWD/build/audio_stream.so"
```

Tap **Enable audio** after connecting. Audio starts at 0 dB boost; the slider can raise it if the simulator output is quiet. High boost can distort loud material; an emergency ceiling is not a substitute for a calibrated mix. The implementation captures actual SDL disk-backend PCM using an opt-in Linux `LD_PRELOAD` library, sends timestamped blocks over a local Unix socket/WebSocket, and plays them through Web Audio with bounded queues and stale-packet rejection. It does not record an unbounded disk stream.

Capture timestamps and bounded scheduling do not prove exact audiovisual synchronization or source-to-speaker latency. Browser autoplay/background interruptions, SDK changes, and network conditions affect playback. Native simulator menus can pause SDK sound. The extraction has a bounded SDK FlippyFish smoke check (startup, streamed frames and browser controls/audio-unlock UI), in addition to the originating Starfield desktop-browser evidence. That is not full FlippyFish playability/audio validation, broad compatibility, or physical iPhone/iPad testing.

## Hosting

The server is a foreground process. Use a process supervisor for persistent hosting, run it as an unprivileged user, and place it behind your own authenticated HTTPS reverse proxy with WebSocket support. Set `-origin` to the exact external origin, including a non-default port. `-listen` defaults to loopback `127.0.0.1:8080`; choose the listener explicitly for your hosting setup.

Origin checking and a single-controller lock are not user authentication. Do not expose the service to the public internet without an access-control layer. The SDK/game execute on the host; load only trusted cartridges. There is no arbitrary file-upload endpoint or multi-user session manager.

`GET /healthz` reports simulator readiness, controller occupancy, and audio availability. Video is PNG over WebSocket rather than an optimized video codec; bandwidth and latency should be assessed on your connection.

## Tests and limitations

```sh
make test
# Optional real private-display transport check (requires Xvfb):
PLAYDATE_BRIDGE_TEST_X11=1 go test -race ./...
```

The included tests cover input edges and crank gestures, session isolation, process cleanup, image conversion, server transport/origin/controller handling, and bounded audio scheduling. Mocked/component tests do not prove compatibility with every game or SDK release. Starfield-specific game scripts, diagnostic recordings, test fixtures, and machine deployment tools are intentionally excluded.

The next compatibility work is full verification beyond the Lua sample smoke check and testing a simulator-compatible C game, improving layout detection, and measuring real iOS behavior. The C audio tap is Linux-specific and sensitive to how the SDK uses SDL. macOS/Windows backends are not implemented.

## Dependencies and license

Uses [Gorilla WebSocket](https://github.com/gorilla/websocket), [XGB](https://github.com/jezek/xgb), Xvfb/X11, Web Audio, and Panic's separately installed simulator. This implementation does not use noVNC. See [THIRD_PARTY.md](THIRD_PARTY.md) and [LICENSE](LICENSE).
