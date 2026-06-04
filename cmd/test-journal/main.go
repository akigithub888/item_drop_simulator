// cmd/test-journal/main.go
// Run from project root: go run ./cmd/test-journal/
// Fetches all encounters for Algeth'ar Academy and merges their loot tables
// so we can compare against KeystoneLoot's flat list.

package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/joho/godotenv"
)

type Instance struct {
	Name       string      `json:"name"`
	Encounters []Encounter `json:"encounters"`
}

type Encounter struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Items []struct {
		Item struct {
			ID   int    `json:"id"`
			Name string `json:"name"`
		} `json:"item"`
	} `json:"items"`
}

func main() {
	_ = godotenv.Load()

	clientID := os.Getenv("BLIZZARD_CLIENT_ID")
	clientSecret := os.Getenv("BLIZZARD_CLIENT_SECRET")
	if clientID == "" || clientSecret == "" {
		fmt.Fprintln(os.Stderr, "BLIZZARD_CLIENT_ID and BLIZZARD_CLIENT_SECRET must be set")
		os.Exit(1)
	}

	token, err := getToken(clientID, clientSecret)
	if err != nil {
		fmt.Fprintf(os.Stderr, "token error: %v\n", err)
		os.Exit(1)
	}

	// Fetch instance to get encounter list
	var instance Instance
	if err := apiGet(token, "/data/wow/journal-instance/1201?namespace=static-us&locale=en_US", &instance); err != nil {
		fmt.Fprintf(os.Stderr, "instance error: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Dungeon: %s (%d encounters)\n\n", instance.Name, len(instance.Encounters))

	// Fetch each encounter and merge loot
	merged := make(map[int]string) // itemID → name
	for _, enc := range instance.Encounters {
		var full Encounter
		path := fmt.Sprintf("/data/wow/journal-encounter/%d?namespace=static-us&locale=en_US", enc.ID)
		if err := apiGet(token, path, &full); err != nil {
			fmt.Fprintf(os.Stderr, "encounter %d error: %v\n", enc.ID, err)
			continue
		}
		fmt.Printf("Boss: %s — %d items\n", full.Name, len(full.Items))
		for _, it := range full.Items {
			merged[it.Item.ID] = it.Item.Name
		}
	}

	fmt.Printf("\nMerged loot pool: %d unique items\n", len(merged))
	fmt.Println("---")
	for id, name := range merged {
		fmt.Printf("  [%d] %s\n", id, name)
	}
}

func getToken(clientID, clientSecret string) (string, error) {
	data := url.Values{}
	data.Set("grant_type", "client_credentials")
	req, err := http.NewRequest("POST", "https://oauth.battle.net/token", strings.NewReader(data.Encode()))
	if err != nil {
		return "", err
	}
	req.SetBasicAuth(clientID, clientSecret)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	var t struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&t); err != nil {
		return "", err
	}
	return t.AccessToken, nil
}

func apiGet(token, path string, out any) error {
	req, err := http.NewRequest("GET", "https://us.api.blizzard.com"+path, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	return json.NewDecoder(resp.Body).Decode(out)
}
