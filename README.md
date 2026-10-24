# Playdate Web Bridge

Run the Playdate simulator on a Linux host and play remotely from your browser, with touch controls, a virtual crank, live video, and audio.

![Starfield running in Playdate Web Bridge](docs/starfield.png)

*[Starfield for Playdate](https://github.com/NickTitle/starfield-playdate), played through the bridge.*

## Quick start over Tailscale

On your Linux host, install Xvfb/X11, Go 1.22+, Make, a C compiler, the [Playdate SDK](https://play.date/dev/), and Tailscale. Copy your simulator-compatible `.pdx` game there.

Connect to the host over SSH:

```sh
git clone https://github.com/NickTitle/playdate-web-bridge.git
cd playdate-web-bridge
make build audio
tailscale serve --bg http://127.0.0.1:8080
```

Copy the HTTPS address printed by Serve into `-origin` below, using just the scheme and hostname:

```sh
export PLAYDATE_SDK_PATH=/path/to/PlaydateSDK
./build/playdate-web-bridge \
  -cartridge /path/to/MyGame.pdx \
  -session "$PWD/session" \
  -origin https://your-host.your-tailnet.ts.net \
  -audio-library "$PWD/build/audio_stream.so"
```

On your laptop or phone connected to the same tailnet, open that HTTPS address. Tap **Connect**, then **Enable audio**. Keep the host command running while you play.

[SSH tunnels and usage](docs/usage.md) · [How does it work?](docs/how-it-works.md) · [Development](docs/development.md) · [License](LICENSE) · [Dependencies](docs/third-party.md)
