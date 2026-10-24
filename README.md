# Playdate Web Bridge

Play Playdate games in your browser through the official Linux simulator, with touch controls, a virtual crank, live 400×240 video, and game audio.

![Starfield running in Playdate Web Bridge](docs/starfield.png)

*[Starfield for Playdate](https://github.com/NickTitle/starfield-playdate), running through the bridge.*

## Quick start

Requires Linux, Xvfb/X11, Go 1.22+, a C compiler, the [Playdate SDK](https://play.date/dev/), and a simulator-compatible `.pdx` game.

```sh
make build audio
export PLAYDATE_SDK_PATH=/path/to/PlaydateSDK
./build/playdate-web-bridge \
  -cartridge /path/to/MyGame.pdx \
  -session "$PWD/session" \
  -origin http://localhost:8080 \
  -audio-library "$PWD/build/audio_stream.so"
```

Open **http://localhost:8080**, tap **Connect**, then **Enable audio**. Use the D-pad, A/B, Menu, and draggable crank. Audio boost starts at 0 dB; raise the slider for quieter games.

Choose a fresh session directory under an existing parent. Saves persist across reconnects; use one session per game. One browser controls the simulator at a time.

## Hosting

Run under a process supervisor as an unprivileged user. For remote access, use an authenticated HTTPS reverse proxy with WebSocket support and set `-origin` to its exact external origin. Load trusted cartridges.

`-listen` defaults to `127.0.0.1:8080`. `GET /healthz` reports simulator readiness, controller occupancy, and audio availability.

## Development

```sh
make test  # Go race checks and Node.js component tests
PLAYDATE_BRIDGE_TEST_X11=1 go test -race ./...  # Private Xvfb check
```

Calibrated for Playdate SDK 3.1.2's default 479×706 simulator layout. Validation covers Starfield desktop-browser use and an SDK FlippyFish startup/rendering smoke test. Broader game, SDK, and mobile-device compatibility is under evaluation.

The bridge uses PNG/WebSocket streaming, X11 input, and a Linux SDL audio-capture hook. Audio timing and quality depend on the host, browser, and network.

[MIT license](LICENSE) · [Third-party components](THIRD_PARTY.md)
