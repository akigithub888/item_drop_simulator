# scrape-wowhead

Builds `data/item_specs.json` from Wowhead's loot-specialization data.
Run once per season, or whenever the dungeon pool changes.

## Setup

The scraper needs `golang.org/x/net` for SOCKS5 support.
Add it to your module from the project root:

```bash
go get golang.org/x/net/proxy
```

## Usage

### Direct (if you can reach Wowhead)
```bash
go run ./cmd/scrape-wowhead/
```

### Through Tor Browser
Start Tor Browser, then:
```bash
go run ./cmd/scrape-wowhead/ --proxy 10.255.255.254:9150
```

### Through system Tor (`tor` package / service)
```bash
go run ./cmd/scrape-wowhead/ --proxy 10.255.255.254:9050
```

### Through system Tor with circuit rotation every 20 items
Requires `ControlPort 9051` in your `torrc` (and `CookieAuthentication 0`):
```bash
go run ./cmd/scrape-wowhead/ --proxy 10.255.255.254:9050 --rotate 20
```

### Brave's built-in Tor window
Open a Private Window with Tor in Brave, then:
```bash
go run ./cmd/scrape-wowhead/ --proxy 127.0.0.1:9150
```
Note: Brave's Tor port may vary — check `brave://net-internals/#proxy` if 9150 doesn't work.

## All flags

| Flag | Default | Description |
|------|---------|-------------|
| `--data` | `./data` | Directory with `items.lua` / `dungeons.lua` |
| `--out` | `./data/item_specs.json` | Output file path |
| `--delay` | `1200` | Milliseconds between requests |
| `--proxy` | _(none)_ | SOCKS5 proxy address |
| `--rotate` | `0` | Rotate Tor circuit every N items (0 = off) |

## Resuming

The scraper writes progress after every single item. If it stops for any reason
(Ctrl-C, network error, rate limit) just run it again — items already in the
JSON are skipped automatically.

## Enabling Tor ControlPort (for --rotate)

Add to `/etc/tor/torrc` (Linux) or your Tor Browser's `torrc`:
```
ControlPort 9051
CookieAuthentication 0
```
Then restart Tor. The scraper connects to port 9051 and sends `SIGNAL NEWNYM`
to request a new exit circuit.