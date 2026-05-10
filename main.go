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
	// Load .env if present (ignored in production where env vars are set directly)
	err := godotenv.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "godotenv error: %v\n", err)
	}

	rng := rand.New(rand.NewSource(time.Now().UnixNano()))

	blizz, err := NewBlizzardClient()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Blizzard client error: %v\n", err)
		os.Exit(1)
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	http.HandleFunc("/health", handleHealth)
	http.HandleFunc("/simulate", handleSimulate(rng))
	http.HandleFunc("/dungeons", handleDungeons(blizz))
	http.HandleFunc("/dungeons/", handleDungeonLoot(blizz))
	http.Handle("/", http.FileServer(http.Dir("frontend")))

	fmt.Printf("Server running on http://localhost:%s\n", port)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		fmt.Fprintf(os.Stderr, "server error: %v\n", err)
		os.Exit(1)
	}
}
