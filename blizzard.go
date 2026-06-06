package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"
)

type BlizzardClient struct {
	clientID     string
	clientSecret string
	region       string
	httpClient   *http.Client

	mu          sync.Mutex
	accessToken string
	tokenExpiry time.Time
}

type blizzardTokenResponse struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int    `json:"expires_in"`
}

type DungeonIndex struct {
	Instances []DungeonRef `json:"instances"`
}

type DungeonRef struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// LootItem is what we expose to the frontend.
type LootItem struct {
	ID            int    `json:"id"`
	Name          string `json:"name"`
	WowheadURL    string `json:"wowhead_url"`
	InventoryType int    `json:"inventory_type"`
}

// MidnightSeason1Dungeons filters the Blizzard journal instance list.
// Keys are Blizzard journal instance IDs.
var MidnightSeason1Dungeons = map[int]bool{
	1299: true,
	1315: true,
	1300: true,
	1316: true,
	1201: true,
	945:  true,
	476:  true,
	278:  true,
}

// BlizzardToLuaInstanceID maps Blizzard journal instance IDs to KeystoneLoot instanceIds.
var BlizzardToLuaInstanceID = map[int]int{
	1299: 2805, // Windrunner Spire
	1315: 2874, // Maisara Caverns
	1300: 2811, // Magisters' Terrace
	1316: 2915, // Nexus-Point Xenas
	1201: 2526, // Algeth'ar Academy
	945:  1753, // Seat of the Triumvirate
	476:  1209, // Skyreach (Dawn of the Infinite)
	278:  658,  // Pit of Saron
}

func NewBlizzardClient() (*BlizzardClient, error) {
	id := os.Getenv("BLIZZARD_CLIENT_ID")
	secret := os.Getenv("BLIZZARD_CLIENT_SECRET")
	if id == "" || secret == "" {
		return nil, fmt.Errorf("BLIZZARD_CLIENT_ID and BLIZZARD_CLIENT_SECRET must be set")
	}
	return &BlizzardClient{
		clientID:     id,
		clientSecret: secret,
		region:       "us",
		httpClient:   &http.Client{Timeout: 10 * time.Second},
	}, nil
}

func (c *BlizzardClient) token() (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.accessToken != "" && time.Now().Before(c.tokenExpiry) {
		return c.accessToken, nil
	}

	data := url.Values{}
	data.Set("grant_type", "client_credentials")

	req, err := http.NewRequest("POST", "https://oauth.battle.net/token", strings.NewReader(data.Encode()))
	if err != nil {
		return "", err
	}
	req.SetBasicAuth(c.clientID, c.clientSecret)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("token request failed: %w", err)
	}
	defer resp.Body.Close()

	var t blizzardTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&t); err != nil {
		return "", fmt.Errorf("token decode failed: %w", err)
	}

	c.accessToken = t.AccessToken
	c.tokenExpiry = time.Now().Add(time.Duration(t.ExpiresIn-60) * time.Second)
	return c.accessToken, nil
}

func (c *BlizzardClient) get(path string, out any) error {
	token, err := c.token()
	if err != nil {
		return err
	}

	reqURL := fmt.Sprintf("https://%s.api.blizzard.com%s", c.region, path)
	req, err := http.NewRequest("GET", reqURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("API request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("API returned status %d for %s", resp.StatusCode, path)
	}

	return json.NewDecoder(resp.Body).Decode(out)
}

// GetDungeons returns the current season dungeon list filtered by Blizzard instance ID.
func (c *BlizzardClient) GetDungeons() ([]DungeonRef, error) {
	var index DungeonIndex
	if err := c.get("/data/wow/journal-instance/index?namespace=static-us&locale=en_US", &index); err != nil {
		return nil, err
	}

	var filtered []DungeonRef
	for _, d := range index.Instances {
		if MidnightSeason1Dungeons[d.ID] {
			filtered = append(filtered, d)
		}
	}
	return filtered, nil
}

// GetItemNames fetches English names for a slice of item IDs concurrently.
// Returns a map of item ID -> name (and slots). Failed fetches are silently skipped.
func (c *BlizzardClient) GetItemDetails(itemIDs []int) (map[int]string, map[int]int) {
	type result struct {
		id            int
		name          string
		inventoryType int
	}

	sem := make(chan struct{}, 10)
	results := make(chan result, len(itemIDs))

	for _, id := range itemIDs {
		id := id
		go func() {
			sem <- struct{}{}
			defer func() { <-sem }()

			var out struct {
				Name          string `json:"name"`
				InventoryType struct {
					Type string `json:"type"` // API returns a string like "FINGER"
				} `json:"inventory_type"`
			}
			inventoryTypeMap := map[string]int{
				"HEAD":        1,
				"NECK":        2,
				"SHOULDER":    3,
				"CHEST":       5,
				"ROBE":        5, // chest variant
				"WAIST":       6,
				"LEGS":        7,
				"FEET":        8,
				"WRIST":       9,
				"HANDS":       10,
				"HAND":        10, // API returns both
				"FINGER":      11,
				"TRINKET":     13,
				"CLOAK":       15,
				"BACK":        15,
				"MAINHAND":    16,
				"WEAPON":      16,
				"TWOHWEAPON":  17,
				"OFFHAND":     17,
				"HOLDABLE":    17, // off-hand held items (tomes, etc.)
				"SHIELD":      17,
				"RANGED":      18,
				"RANGEDRIGHT": 18, // guns/bows/crossbows
				"NON_EQUIP":   0,  // explicitly non-equippable, will be hidden in frontend
			}
			path := fmt.Sprintf("/data/wow/item/%d?namespace=static-us&locale=en_US", id)
			if err := c.get(path, &out); err == nil && out.Name != "" {
				slot := inventoryTypeMap[out.InventoryType.Type]
				results <- result{id, out.Name, slot}
			}
		}()
	}

	names := make(map[int]string, len(itemIDs))
	slots := make(map[int]int, len(itemIDs))
	for range itemIDs {
		r := <-results
		if r.name != "" {
			names[r.id] = r.name
			slots[r.id] = r.inventoryType
		}
	}
	return names, slots
}
