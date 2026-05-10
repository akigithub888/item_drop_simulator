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

// BlizzardClient handles auth and requests to the Blizzard Game Data API.
type BlizzardClient struct {
	clientID     string
	clientSecret string
	region       string
	httpClient   *http.Client

	mu          sync.Mutex
	accessToken string
	tokenExpiry time.Time
}

// --- Auth token types ---

type blizzardTokenResponse struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int    `json:"expires_in"`
}

// --- Journal API types ---

type DungeonIndex struct {
	Instances []DungeonRef `json:"instances"`
}

type DungeonRef struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

type DungeonDetail struct {
	ID         int         `json:"id"`
	Name       string      `json:"name"`
	Encounters []Encounter `json:"encounters"`
}

type Encounter struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

type EncounterDetail struct {
	ID    int         `json:"id"`
	Name  string      `json:"name"`
	Items []LootEntry `json:"items"`
}

type LootEntry struct {
	ID   int `json:"id"`
	Item struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	} `json:"item"`
}

// LootItem is what we expose to the frontend.
type LootItem struct {
	ID         int    `json:"id"`
	Name       string `json:"name"`
	WowheadURL string `json:"wowhead_url"`
}

// NewBlizzardClient creates a client from environment variables.
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

// token returns a valid access token, fetching a new one if needed.
func (c *BlizzardClient) token() (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.accessToken != "" && time.Now().Before(c.tokenExpiry) {
		return c.accessToken, nil
	}

	data := url.Values{}
	data.Set("grant_type", "client_credentials")

	req, err := http.NewRequest("POST",
		"https://oauth.battle.net/token",
		strings.NewReader(data.Encode()),
	)
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

// get makes an authenticated GET request to the Blizzard API.
func (c *BlizzardClient) get(path string, out any) error {
	token, err := c.token()
	if err != nil {
		return err
	}

	base := fmt.Sprintf("https://%s.api.blizzard.com", c.region)
	reqURL := fmt.Sprintf("%s%s", base, path)

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

// GetDungeons returns the list of all journal instances (dungeons + raids).
func (c *BlizzardClient) GetDungeons() ([]DungeonRef, error) {
	var index DungeonIndex
	err := c.get("/data/wow/journal-instance/index?namespace=static-us&locale=en_US", &index)
	if err != nil {
		return nil, err
	}
	return index.Instances, nil
}

// GetLoot returns the combined loot table for all encounters in a dungeon.
func (c *BlizzardClient) GetLoot(dungeonID int) ([]LootItem, error) {
	var detail DungeonDetail
	path := fmt.Sprintf("/data/wow/journal-instance/%d?namespace=static-us&locale=en_US", dungeonID)
	if err := c.get(path, &detail); err != nil {
		return nil, err
	}

	seen := map[int]bool{}
	var items []LootItem

	for _, enc := range detail.Encounters {
		var encDetail EncounterDetail
		encPath := fmt.Sprintf("/data/wow/journal-encounter/%d?namespace=static-us&locale=en_US", enc.ID)
		if err := c.get(encPath, &encDetail); err != nil {
			continue // skip encounters that fail, don't abort everything
		}
		for _, entry := range encDetail.Items {
			if seen[entry.Item.ID] || entry.Item.Name == "" {
				continue
			}
			seen[entry.Item.ID] = true
			items = append(items, LootItem{
				ID:         entry.Item.ID,
				Name:       entry.Item.Name,
				WowheadURL: fmt.Sprintf("https://www.wowhead.com/item=%d", entry.Item.ID),
			})
		}
	}

	return items, nil
}
