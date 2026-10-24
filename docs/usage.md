# Remote usage

The Linux host runs the SDK, cartridge, and bridge. Your laptop or phone supplies the browser and controls.

## Tailscale HTTPS

Connect both devices to your tailnet. On the host, [Tailscale Serve](https://tailscale.com/docs/reference/tailscale-cli/serve) forwards HTTPS traffic to the bridge:

```sh
tailscale serve --bg http://127.0.0.1:8080
```

Use the printed HTTPS address as the bridge's `-origin`, for example `https://playdate-host.tailxxxx.ts.net`. Use just the scheme and hostname; include the port for a custom HTTPS listener. Open that same address on your client device. Serve handles HTTPS, WebSocket forwarding, and tailnet access controls.

For a dedicated port, use `tailscale serve --bg --https=8443 http://127.0.0.1:8080` and include `:8443` in both `-origin` and the browser address.

The backend stays on its default loopback listener, `127.0.0.1:8080`. Tailnet access is governed by your Tailscale policy.

## SSH tunnel

For a laptop with SSH access, start the bridge on the Linux host:

```sh
export PLAYDATE_SDK_PATH=/path/to/PlaydateSDK
./build/playdate-web-bridge \
  -cartridge /path/to/MyGame.pdx \
  -session "$PWD/session" \
  -origin http://localhost:8080 \
  -audio-library "$PWD/build/audio_stream.so"
```

Then run this on your laptop:

```sh
ssh -N -L 8080:127.0.0.1:8080 user@linux-host
```

Open **http://localhost:8080** on the laptop. That address reaches the remote bridge through the [SSH tunnel](https://man.openbsd.org/ssh#L). Keep both the tunnel and host process running.

If you choose client port 8081, use `ssh -N -L 8081:127.0.0.1:8080 user@linux-host`, open `http://localhost:8081`, and set the host's `-origin` to `http://localhost:8081`.

## Controls, sound, and saves

Tap **Connect** to take control. The D-pad sits on the left, with round B/A buttons on the right and Menu above the screen at the right edge. The crank has its own dial. Button actions follow the loaded game. One browser controls the session at a time. Disconnects and expired inputs release held controls.

Tap **Enable audio** to unlock [browser audio playback](https://developer.mozilla.org/en-US/docs/Web/API/Web_Audio_API/Best_practices). Tap the sound button again to pause/resume. Boost starts at 0 dB; raise it for quiet output. Higher boost can distort loud material. Native simulator menus pause SDK sound and resume it on close.

Choose a fresh session directory under an existing parent. Saves and settings persist across reconnects; use a separate directory for each game.

## Persistent hosting and connection checks

Run the host command under a process supervisor such as systemd, as an unprivileged user. Use trusted cartridges and control access through SSH, Tailscale, or an authenticated HTTPS reverse proxy with WebSocket forwarding.

Set `-origin` to the exact browser origin: scheme, hostname, and port, with a bare origin rather than a path. `localhost` and `127.0.0.1` are different origins. The browser automatically uses the corresponding WebSocket scheme.

`GET /healthz` reports simulator readiness, controller occupancy, and audio availability. For an SSH tunnel:

```sh
curl http://localhost:8080/healthz
```

For Tailscale, use your HTTPS address followed by `/healthz`. Video bandwidth and audio timing depend on the host, browser, and connection.

[Back to quick start](../README.md)
