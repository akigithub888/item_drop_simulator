# Item Drop Simulator

A Go-based drop chance simulator for dungeon loot. The server loads dungeon and item metadata from `data/`, resolves item names via the Blizzard API, and exposes a JSON API plus a simple frontend for exploring loot pools, estimating drop chances, and simulating runs.

The app is also live at: https://item-drop-simulator.onrender.com

This simulator is built for World of Warcraft dungeon loot. It uses dungeon / item metadata from the `data/` directory and supports the current seasonal dungeon pool and item drop rules.

## Features

- Simulate drop probability for a target item in a custom loot pool
- Account for group size and traders who can obtain the item and trade it to you
- Compute expected runs and probability curves for common thresholds
- Serve dungeon and loot metadata via HTTP
- Parse SimulationCraft strings for class/spec lookup
- Includes a browser-based frontend at `/`

## Requirements

- Go 1.20+ (or compatible Go toolchain)
- `data/` directory with required metadata:
  - `items.lua`
  - `dungeons.lua`
  - `item_specs.json`
  - `trinket_overrides.json`
- Internet access for Blizzard API item name resolution at startup

## Data sources and replication

The simulator reads World of Warcraft dungeon loot metadata from the `data/` directory. The files include:

- `items.lua`: raw item definitions and IDs
- `dungeons.lua`: dungeon IDs, seasons, and loot zone mappings
- `item_specs.json`: specialization-specific loot pool rules
- `trinket_overrides.json`: special-case item mapping overrides

At startup, the app also resolves item names through the Blizzard API, so the raw data IDs are mapped to human-readable item names.

If you want to reproduce the data locally, the repository includes a scraper helper in `cmd/scrape-wowhead/` that can generate or refresh item/spec metadata from Wowhead.

## Run locally

Clone the repository and run it from your local machine:

```bash
git clone <repository-url>
cd item_drop_simulator
go run .
```

Or build and run the binary:

```bash
go build -o item_drop_simulator .
./item_drop_simulator
```

The server listens on `localhost:8080` by default. To use a custom port:

```bash
PORT=3000 go run .
```

If you need to refresh the WoW loot dataset, update the files in `data/` and regenerate `item_specs.json` using the helper under `cmd/scrape-wowhead/`.

## HTTP API

### Health check

- `GET /health`

Returns:

```json
{"status":"ok"}
```

### Simulate drop chances

- `POST /simulate`
- Request body:

```json
{
  "target_item": "Mythic Sword",
  "players": 5,
  "traders": 2,
  "pool": ["Item A", "Item B", "Mythic Sword"],
  "trials": 100000
}
```

- Response body contains:
  - `drop_chance`
  - `expected_runs`
  - `simulated_runs`
  - `trials`
  - probability curve points

### Dungeon metadata

- `GET /dungeons`
- `GET /dungeons/{blizzard_dungeon_id}?spec={specID}`

Use these endpoints to fetch available dungeon data and lookup spec-specific loot.

### Parse SimulationCraft strings

- `POST /parse-simc`
- Request body:

```json
{ "simc": "<your simc string>" }
```

- Response body includes class/spec identifiers and resolved names.

## Frontend

The repository serves `frontend/index.html` at `/`. Open the app in your browser after starting the server.

## Data

The simulator reads dungeon and loot metadata from the `data/` directory. This includes:

- `items.lua` — item definitions and identifiers
- `dungeons.lua` — dungeon IDs and maps
- `item_specs.json` — spec-specific loot pools
- `trinket_overrides.json` — special item mapping rules

## Notes

The main application entrypoint is the server in `main.go`. There is also a separate helper under `cmd/scrape-wowhead/` in this repository, but the simulator itself is focused on running drop simulations and serving loot data.
