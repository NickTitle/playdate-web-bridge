# Remote usage

The Linux host runs the SDK, cartridge, and bridge. Your laptop or phone supplies the browser and controls. Choose SSH forwarding, Tailscale, your own reverse proxy, or private-network access.

Before launching the [quick-start command](../README.md#quick-start), set `BRIDGE_ORIGIN` on the host to the exact origin your browser will open: scheme, hostname, and port. Use a bare origin. `localhost` and `127.0.0.1` are different origins. Changing the client address requires relaunching the bridge with the matching origin.

## SSH forwarding

Set this on the Linux host before launching the bridge:

```sh
export BRIDGE_ORIGIN=http://localhost:8080
```

On your client laptop, open a separate terminal and establish the [SSH tunnel](https://man.openbsd.org/ssh#L):

```sh
ssh -N -L 8080:127.0.0.1:8080 user@linux-host
```

Open **http://localhost:8080** on the laptop. SSH carries that local connection to the remote simulator. Keep the tunnel and host process running.

For client port 8081, use `ssh -N -L 8081:127.0.0.1:8080 user@linux-host`, open `http://localhost:8081`, and set the host's `BRIDGE_ORIGIN` to `http://localhost:8081`. The remote listener stays at `127.0.0.1:8080`.

## Tailscale HTTPS

Connect the host and client to your tailnet. On the host, [Tailscale Serve](https://tailscale.com/docs/reference/tailscale-cli/serve) forwards HTTPS and WebSocket traffic to the bridge:

```sh
tailscale serve --bg http://127.0.0.1:8080
```

Use the printed HTTPS origin when launching the bridge:

```sh
export BRIDGE_ORIGIN=https://your-host.your-tailnet.ts.net
```

Open that HTTPS address on your tailnet-connected laptop or phone. Tailnet access follows your Tailscale policy.

A dedicated listener can use `tailscale serve --bg --https=8443 http://127.0.0.1:8080`; include `:8443` in the origin and browser address.

## Reverse proxy or private network

For your own HTTPS endpoint, forward HTTP and WebSocket upgrades to the backend at `127.0.0.1:8080`, and set:

```sh
export BRIDGE_ORIGIN=https://playdate.example.com
```

Open that address on your client. Use proxy authentication or VPN policy to control access. The browser selects `wss:` automatically for HTTPS pages.

For direct access over a private network, bind the bridge to an address on that network using `-listen`, and match `-origin` to the browser address. For example, with host VPN address `10.8.0.5`:

```sh
export BRIDGE_ORIGIN=http://10.8.0.5:8080
# Add -listen 10.8.0.5:8080 to the quick-start bridge command.
```

Restrict access through your VPN/firewall policy. An HTTPS proxy supplies encryption and a secure browser context for remote clients. The default loopback listener is suitable for SSH and proxies.

## Controls, sound, and saves

Tap **Connect** to take control. The D-pad sits left, with round B/A buttons right and Menu at the upper-right edge. The crank has its own dial. Button actions follow the loaded game. One browser controls a session at a time; disconnects and expired inputs release held controls.

Tap **Enable audio** to unlock [browser playback](https://developer.mozilla.org/en-US/docs/Web/API/Web_Audio_API/Best_practices); tap again to pause/resume. Boost starts at 0 dB. Raise it for quiet output; high boost can distort louder material. Native simulator menus pause SDK sound and resume it on close.

Choose a fresh session directory under an existing parent. Saves and settings persist across reconnects; use a separate directory for each game.

## Persistent hosting and checks

Run the host command under a process supervisor such as systemd, as an unprivileged user. Use trusted cartridges. Keep connection credentials and access controls in your chosen SSH, VPN, or proxy setup.

`GET /healthz` reports simulator readiness, controller occupancy, and audio availability. Check it through the same client connection used for play, for example:

```sh
curl http://localhost:8080/healthz  # Through the SSH tunnel
curl https://playdate.example.com/healthz  # Through your HTTPS endpoint
```

Video bandwidth and audio timing depend on the host, browser, and connection.

[Back to quick start](../README.md)
