package main

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
)

// ItemEntry holds per-item spec eligibility.
type ItemEntry struct {
	ItemID  int
	SpecIDs map[int]bool
}

// DungeonEntry maps a Lua instanceId to its loot table item IDs.
type DungeonEntry struct {
	InstanceID      int
	ChallengeModeID int
	ItemIDs         []int
}

// LootData holds all parsed data from the Lua files.
type LootData struct {
	Items        map[int]ItemEntry
	Dungeons     map[int]DungeonEntry
	ItemNames    map[int]string
	ItemSlots    map[int]int             // itemID -> inventory_type
	TrinketSpecs map[string]map[int]bool // itemName -> set of allowed specIDs
}

var reNumbers = regexp.MustCompile(`\d+`)

// LoadLootData parses items.lua and dungeons.lua, then overlays spec data from
// item_specs.json if present.  The JSON (produced by cmd/scrape-wowhead) is
// authoritative: it comes from Wowhead's own loot-specialization data rather
// than KeystoneLoot's "can equip" heuristic, so it is both more accurate and
// handles edge cases like the Algeth'ar Puzzle Box correctly.
func LoadLootData(dataDir string) (*LootData, error) {
	ld := &LootData{
		Items:        make(map[int]ItemEntry),
		Dungeons:     make(map[int]DungeonEntry),
		ItemNames:    make(map[int]string),
		ItemSlots:    make(map[int]int),
		TrinketSpecs: make(map[string]map[int]bool),
	}

	if err := ld.parseItems(dataDir + "/items.lua"); err != nil {
		return nil, fmt.Errorf("items.lua: %w", err)
	}
	if err := ld.parseDungeons(dataDir + "/dungeons.lua"); err != nil {
		return nil, fmt.Errorf("dungeons.lua: %w", err)
	}

	// Overlay Wowhead spec data if the JSON file exists.
	specsPath := dataDir + "/item_specs.json"
	if loaded, n, err := ld.loadWowheadSpecs(specsPath); err != nil {
		fmt.Fprintf(os.Stderr, "[WARN] could not load %s: %v — using KeystoneLoot spec data\n", specsPath, err)
	} else if loaded {
		fmt.Printf("Loaded Wowhead spec data for %d items from %s\n", n, specsPath)
	}

	// Load trinket role overrides.
	overridesPath := dataDir + "/trinket_overrides.json"
	if data, err := os.ReadFile(overridesPath); err == nil {
		var raw map[string][]int
		if json.Unmarshal(data, &raw) == nil {
			ld.TrinketSpecs = make(map[string]map[int]bool)
			for name, specList := range raw {
				ld.TrinketSpecs[name] = make(map[int]bool, len(specList))
				for _, sid := range specList {
					ld.TrinketSpecs[name][sid] = true
				}
			}
			fmt.Printf("Loaded trinket overrides for %d items from %s\n", len(ld.TrinketSpecs), overridesPath)
		}
	}

	return ld, nil
}

// loadWowheadSpecs reads item_specs.json and overwrites SpecIDs for every
// item that appears in it.  Returns (fileExisted, itemsUpdated, error).
//
// JSON format (produced by cmd/scrape-wowhead):
//
//	{ "193701": [103, 255, 577, ...], "193702": [], ... }
//
// An empty slice means "no spec restriction" (drops for everyone).
// A missing key means the item was not scraped — we keep KeystoneLoot data.
func (ld *LootData) loadWowheadSpecs(path string) (bool, int, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, 0, nil // file simply doesn't exist yet — silently skip
		}
		return false, 0, err
	}

	raw := make(map[string][]int)
	if err := json.Unmarshal(data, &raw); err != nil {
		return true, 0, fmt.Errorf("parse error: %w", err)
	}

	updated := 0
	for keyStr, specList := range raw {
		itemID, err := strconv.Atoi(keyStr)
		if err != nil {
			continue
		}

		entry, exists := ld.Items[itemID]
		if !exists {
			// Item may appear in dungeons.lua but not items.lua; create a stub.
			entry = ItemEntry{ItemID: itemID, SpecIDs: make(map[int]bool)}
		}

		if len(specList) == 0 {
			// Empty list → item has no spec restriction; clear any previous data.
			entry.SpecIDs = make(map[int]bool)
		} else {
			entry.SpecIDs = make(map[int]bool, len(specList))
			for _, sid := range specList {
				entry.SpecIDs[sid] = true
			}
		}

		ld.Items[itemID] = entry
		updated++
	}

	return true, updated, nil
}

func (ld *LootData) parseItems(path string) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	itemIDRe := regexp.MustCompile(`^\s*\[(\d+)\]\s*=\s*\{`)
	classesRe := regexp.MustCompile(`classes\s*=\s*\{((?:[^{}]|\{[^{}]*\})*)\}`)
	classEntryRe := regexp.MustCompile(`\[(\d+)\]\s*=\s*\{([^}]*)\}`)

	lines := strings.Split(string(raw), "\n")
	var block strings.Builder
	depth := 0
	currentItemID := 0

	for _, line := range lines {
		if depth == 0 {
			m := itemIDRe.FindStringSubmatch(line)
			if m != nil {
				currentItemID, _ = strconv.Atoi(m[1])
				block.Reset()
			}
		}
		if currentItemID == 0 {
			continue
		}

		block.WriteString(line)
		depth += strings.Count(line, "{") - strings.Count(line, "}")

		if depth <= 0 {
			entry := ItemEntry{ItemID: currentItemID, SpecIDs: make(map[int]bool)}
			cm := classesRe.FindStringSubmatch(block.String())
			if cm != nil {
				for _, match := range classEntryRe.FindAllStringSubmatch(cm[1], -1) {
					for _, s := range reNumbers.FindAllString(match[2], -1) {
						id, _ := strconv.Atoi(s)
						entry.SpecIDs[id] = true
					}
				}
			}
			ld.Items[currentItemID] = entry
			currentItemID = 0
			depth = 0
			block.Reset()
		}
	}

	return nil
}

func (ld *LootData) parseDungeons(path string) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	stripComment := regexp.MustCompile(`--\[\[.*?\]\]`)
	challengeRe := regexp.MustCompile(`challengeModeId\s*=\s*(\d+)`)
	instanceRe := regexp.MustCompile(`instanceId\s*=\s*(\d+)`)
	lootRe := regexp.MustCompile(`lootTable\s*=\s*\{([^}]+)\}`)

	for _, line := range strings.Split(string(raw), "\n") {
		line = stripComment.ReplaceAllString(line, "")
		if !strings.Contains(line, "instanceId") {
			continue
		}

		cmMatch := challengeRe.FindStringSubmatch(line)
		instMatch := instanceRe.FindStringSubmatch(line)
		ltMatch := lootRe.FindStringSubmatch(line)

		if cmMatch == nil || instMatch == nil || ltMatch == nil {
			continue
		}

		challengeModeID, _ := strconv.Atoi(cmMatch[1])
		instanceID, _ := strconv.Atoi(instMatch[1])

		var itemIDs []int
		for _, s := range reNumbers.FindAllString(ltMatch[1], -1) {
			id, _ := strconv.Atoi(s)
			itemIDs = append(itemIDs, id)
		}

		ld.Dungeons[instanceID] = DungeonEntry{
			InstanceID:      instanceID,
			ChallengeModeID: challengeModeID,
			ItemIDs:         itemIDs,
		}
	}

	return nil
}

// AllItemIDs returns a deduplicated slice of all item IDs across the current season dungeons.
func (ld *LootData) AllItemIDs(blizzardToLua map[int]int) []int {
	seen := make(map[int]bool)
	var ids []int
	for _, luaID := range blizzardToLua {
		dungeon, ok := ld.Dungeons[luaID]
		if !ok {
			continue
		}
		for _, id := range dungeon.ItemIDs {
			if !seen[id] {
				seen[id] = true
				ids = append(ids, id)
			}
		}
	}
	return ids
}

var TrinketSpecs map[string]map[int]bool // item name -> set of allowed spec IDs

func (ld *LootData) GetLootForSpec(luaInstanceID, specID int) ([]LootItem, error) {
	dungeon, ok := ld.Dungeons[luaInstanceID]
	if !ok {
		return nil, fmt.Errorf("dungeon instance %d not found in loot data", luaInstanceID)
	}

	var items []LootItem
	for _, itemID := range dungeon.ItemIDs {
		entry, hasEntry := ld.Items[itemID]
		if specID != 0 {
			slot := ld.ItemSlots[itemID]
			isRing := slot == 11 || slot == 12
			isTrinket := slot == 13 || slot == 14

			if isTrinket {
				name := ld.ItemNames[itemID]
				if specs, ok := ld.TrinketSpecs[name]; ok {
					if !specs[specID] {
						continue
					}
				} else {
					// not in override list, fall back to spec data
					if !hasEntry || len(entry.SpecIDs) == 0 || !entry.SpecIDs[specID] {
						continue
					}
				}
			} else if !isRing {
				if !hasEntry || len(entry.SpecIDs) == 0 || !entry.SpecIDs[specID] {
					continue
				}
			}
		}

		name := ld.ItemNames[itemID]
		if name == "" {
			name = fmt.Sprintf("Item #%d", itemID)
		}

		items = append(items, LootItem{
			ID:            itemID,
			Name:          name,
			WowheadURL:    fmt.Sprintf("https://www.wowhead.com/item=%d", itemID),
			InventoryType: ld.ItemSlots[itemID],
		})
	}

	return items, nil
}
