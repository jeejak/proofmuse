# Deploying the demo

One container, no database. Results (never photos) are cached in a volume, so a restart keeps them and reruns of the same photos stay free.

## Build and run

```bash
docker build -t proofmuse .
docker run -d --name proofmuse --restart unless-stopped \
  -p 127.0.0.1:8080:8080 \
  -v proofmuse-data:/data \
  -e MODEL_API_KEY="$MODEL_API_KEY" \
  -e TRUST_PROXY=1 \
  -e LIMIT_PER_HOUR=30 -e LIMIT_PER_DAY=500 \
  proofmuse
curl -s localhost:8080/health   # ok
```

- `MODEL_API_KEY` comes from the host environment or a secrets file with mode 600. It is never baked into the image or committed.
- `TRUST_PROXY=1` only behind a proxy that sets `X-Forwarded-For` (Cloudflare does). Without a proxy, leave it unset, or one client can spoof its address past the rate limit.
- `LIMIT_PER_HOUR` caps new checks per client per hour; `LIMIT_PER_DAY` caps new checks across everyone. At about $0.014 per check, 500 a day is about $7. Cached results and the sample pages do not count.
- Bind to `127.0.0.1` so the only way in is the tunnel.

## Public hostname through a Cloudflare tunnel

Use a one-level name such as `proofmuse.jeejak.id`. Cloudflare's Universal SSL covers `*.jeejak.id` only one level deep, so a name like `demo.proofmuse.jeejak.id` fails the TLS handshake.

```bash
cloudflared tunnel create proofmuse
cloudflared tunnel route dns proofmuse proofmuse.jeejak.id
```

Tunnel config (`/etc/cloudflared/proofmuse.yml`):

```yaml
tunnel: proofmuse
credentials-file: /etc/cloudflared/proofmuse.json
ingress:
  - hostname: proofmuse.jeejak.id
    service: http://localhost:8080
  - service: http_status:404
```

Run it as a service (`cloudflared --config /etc/cloudflared/proofmuse.yml service install`, or a systemd unit with `Restart=always`).

## Before the demo and the judging window

1. `curl -s https://proofmuse.jeejak.id/health` answers `ok`.
2. Open the page on a phone and run every sample once, so the recorded and judged runs come from cached results.
3. Set a spend-limit notification on the Meta Model API Billing page.
4. Keep it up from submission (Nov 4) through judging (Nov 5 to 8) and the finals (Nov 9).
