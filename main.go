package main

import (
	"fmt"
	"math/rand"
	"net/http"
	"os"
	"time"

	"github.com/joho/godotenv"
)

func main() {
	_ = godotenv.Load()

	rng := rand.New(rand.NewSource(time.Now().UnixNano()))

	blizz, err := NewBlizzardClient()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Blizzard client error: %v\n", err)
		os.Exit(1)
	}

	loot, err := LoadLootData("data")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Loot data error: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Loaded %d items and %d dungeons from loot data\n", len(loot.Items), len(loot.Dungeons))

	// Fetch item names from Blizzard API for all known season dungeons
	fmt.Println("Fetching item names from Blizzard API...")
	allItemIDs := loot.AllItemIDs(BlizzardToLuaInstanceID)
	names, slots := blizz.GetItemDetails(loot.AllItemIDs(BlizzardToLuaInstanceID))
	loot.ItemNames = names
	loot.ItemSlots = slots
	loot.ItemNames = names
	fmt.Printf("Resolved %d/%d item names\n", len(names), len(allItemIDs))

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	http.HandleFunc("/health", handleHealth)
	http.HandleFunc("/simulate", handleSimulate(rng))
	http.HandleFunc("/dungeons", handleDungeons(blizz))
	http.HandleFunc("/dungeons/", handleDungeonLoot(loot))
	http.HandleFunc("/parse-simc", handleParseSimC)
	http.Handle("/", http.FileServer(http.Dir("frontend")))

	fmt.Printf("Server running on http://localhost:%s\n", port)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		fmt.Fprintf(os.Stderr, "server error: %v\n", err)
		os.Exit(1)
	}
}
