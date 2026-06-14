# Dungeon Drop Dive

A World of Warcraft dungeon loot simulator for estimating drop chances, expected runs, and spec-specific loot pools. The app is live at: https://item-drop-simulator.onrender.com

## Description

Dungeon Drop Dive uses WoW dungeon and item metadata to simulate how likely it is to obtain a specific drop from a season's dungeon pool. It calculates probability curves, expected clear counts, and the impact of group size and traders.

## Motivation

I built this project to make WoW dungeon loot odds easier to understand. Instead of guessing how many clears it takes to see a target item, Dungeon Drop Dive provides a data-driven view of drop probabilities using real dungeon loot metadata and Blizzard API item resolution.

## Quick Start

### Run the live demo

Open the live app:

- https://item-drop-simulator.onrender.com

### Run locally

```bash
git clone <repository-url>
cd item_drop_simulator
go run .
```

Or build the binary:

```bash
go build -o item_drop_simulator .
./item_drop_simulator
```

The server listens on `localhost:8080` by default. To use a different port:

```bash
PORT=3000 go run .
```

## Usage

### What it does

- Simulates WoW dungeon drop probability for a target item
- Uses the current dungeon loot pool and item metadata
- Accounts for players and traders who can also obtain the item and trade it
- Exposes a JSON API and a simple browser frontend

### HTTP API

#### Health check

- `GET /health`

Returns:

```json
{"status":"ok"}
```

#### Simulate drop chances

- `POST /simulate`

Request body example:

```json
{
  "target_item": "Mythic Sword",
  "players": 5,
  "traders": 2,
  "pool": ["Item A", "Item B", "Mythic Sword"],
  "trials": 100000
}
```

Response body includes:

- `drop_chance`
- `expected_runs`
- `simulated_runs`
- `trials`
- `curve` points for probability thresholds

#### Dungeon metadata

- `GET /dungeons`
- `GET /dungeons/{blizzard_dungeon_id}?spec={specID}`

These endpoints return available dungeon data and spec-specific loot pools.

#### Parse SimulationCraft strings

- `POST /parse-simc`

Request body example:

```json
{ "simc": "<your simc string>" }
```

Response body returns parsed class/spec identifiers and resolved names.

### Frontend

The app serves `frontend/index.html` at `/`. Open the browser after starting the server to use the interactive UI.

## Data

The simulator depends on World of Warcraft dungeon and loot metadata stored in `data/`:

- `items.lua` — item definitions and IDs
- `dungeons.lua` — dungeon IDs, seasonal pools, and loot mappings
- `item_specs.json` — specialization-specific loot pool rules
- `trinket_overrides.json` — special item mapping overrides

At startup, the server resolves item names through the Blizzard API so the raw IDs become readable item titles in the UI and API responses.

If you want to reproduce or refresh the dataset locally, use the helper in `cmd/scrape-wowhead/` to generate or update `item_specs.json` from Wowhead data.

## Contributing

### Clone the repo

```bash
git clone <repository-url>
cd item_drop_simulator
```

### Build and run

```bash
go build
./item_drop_simulator
```

### Run tests

There is no test suite included yet, but the server can be validated by starting it and using the `/health` endpoint.

### Want to contribute?

If you would like to help improve the project, please fork the repository, make your changes, and open a pull request.

## Notes

- The main application entrypoint is `main.go`.
- The project includes a separate `cmd/scrape-wowhead/` helper for generating item/spec metadata from Wowhead.
- The live deployed demo is available at https://item-drop-simulator.onrender.com
