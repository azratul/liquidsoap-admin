# liquidsoap-admin

Web admin interface for a [Liquidsoap](https://www.liquidsoap.info/) radio station. Browse your music library, manage the manual queue, and monitor what's on air — all from the browser.

## Features

- **Now playing** — current track, artist, album art (via Last.fm), live/auto status and uptime
- **Library browser** — navigate your music directory and queue tracks instantly
- **Queue management** — view, flush, or skip the manual queue
- **Search** — case-insensitive substring search across the entire library
- **Responsive** — works on mobile

## Liquidsoap setup

Compatible with **Liquidsoap 2.x**.

liquidsoap-admin connects to Liquidsoap's telnet API and expects two custom commands to be registered in your script, plus a `request.queue` with a known ID:

```liquidsoap
# Enable telnet API
settings.server.telnet         := true
settings.server.telnet.port    := 1234
settings.server.telnet.bind_addr := "0.0.0.0"

# Manual queue — the id must match QUEUE_NAME
manual = request.queue(id="manual", conservative=true)

# ...your sources, fallbacks, outputs, etc...

radio = fallback(track_sensitive=false, [live, manual, radio, base])
radio = mksafe(id="radio", radio)

# Required: exposes current track info to radio-web
def radio_on_air(_) =
  m = null.get(default=[], radio.last_metadata())

  artist = list.assoc(default="", "artist", m)
  title  = list.assoc(default="", "title", m)
  fname  = list.assoc(default="", "filename", m)
  ctype  = list.assoc(default="", "sc_content_type", m)

  "#{artist}|#{title}|#{fname}|#{ctype}"
end

radio.register_command(
  usage="on_air",
  description="Current track: artist|title|filename|sc_content_type",
  "on_air",
  radio_on_air
)

# Required: allows radio-web to remove individual requests from the queue
def manual_remove(arg) =
  rid  = string.trim(arg)
  keep = list.filter(fun(r) -> "#{request.id(r)}" != rid, manual.queue())
  uris = list.map(fun(r) -> request.uri(r), keep)
  manual.set_queue([])
  list.iter(fun(uri) -> manual.push.uri(uri), uris)
  "OK"
end

server.register(
  usage="manual.remove <rid>",
  description="Remove a request from the manual queue by RID",
  "manual.remove",
  manual_remove
)
```

The `uptime`, `manual.push`, `manual.queue`, `manual.flush`, and `skip` commands are built into Liquidsoap and require no extra configuration.

## Deployment

liquidsoap-admin is designed to run as a **sidecar container** alongside Liquidsoap. This way Liquidsoap's telnet port (1234) stays on the internal Docker network and is never exposed to the host.

It can still be used without a sidecar container, but in that case, if Liquidsoap is running in Docker, its telnet port would need to be exposed to the host.

```yaml
services:
  liquidsoap:
    image: savonet/liquidsoap:v2.3.2
    container_name: liquidsoap
    volumes:
      - ./script.liq:/script.liq:ro
      - /your/music:/music:ro
      - /your/jingles:/jingles:ro
    command: ["/script.liq"]
    networks:
      - radio

  icecast:
    image: ghcr.io/azratul/icecast2:latest
    container_name: icecast
    env_file:
      - .env
    ports:
      - "8000:8000"
    volumes:
      - ./icecast2/config:/etc/icecast2
      - ./icecast2/logs:/var/log/icecast2
    networks:
      - radio

  admin:
    image: ghcr.io/azratul/liquidsoap-admin:latest
    ports:
      - "8010:8010"
    volumes:
      - /your/music:/music:ro # The same music directory mounted in liquidsoap
    environment:
      LIQUIDSOAP_ADDR: "liquidsoap:1234"
      MUSIC_ROOT:      "/music"
      QUEUE_NAME:      "manual"
      HTTP_PORT:       "8010"
      # LASTFM_APIKEY: "..."
      # AUTH_USER:     "operator"
      # AUTH_PASS:     "changeme"
    depends_on:
      - liquidsoap
    networks:
      - radio

networks:
  radio:
    driver: bridge
```

## Configuration

| Variable | Default | Purpose |
|---|---|---|
| `LIQUIDSOAP_ADDR` | `liquidsoap:1234` | Liquidsoap telnet address |
| `MUSIC_ROOT` | `/music` | Music library mount path |
| `QUEUE_NAME` | `manual` | Liquidsoap queue ID — must match the `id` of your `request.queue` |
| `HTTP_PORT` | `8010` | Web UI port |
| `LOG_LEVEL` | `info` | Log verbosity: `debug`, `info`, `warn`, `error` |
| `LASTFM_APIKEY` | — | Last.fm API key. When set, album art is fetched and cached per track (optional) |
| `LASTFM_URL` | `https://ws.audioscrobbler.com/2.0` | Last.fm API base URL. Override only if proxying the API (optional) |
| `AUTH_USER` / `AUTH_PASS` | — | When both are set, the entire UI is protected with HTTP Basic Auth (optional) |

## Stack

- **Go** + **[chi](https://github.com/go-chi/chi)**
- **[HTMX](https://htmx.org/)**
- **[Last.fm API](https://www.last.fm/api)** (optional)
