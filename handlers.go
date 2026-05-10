package main

import (
	"encoding/json"
	"math/rand"
	"net/http"
	"strconv"
	"strings"
)

// SimulateRequest is the expected JSON body for POST /simulate.
type SimulateRequest struct {
	TargetItem string   `json:"target_item"`
	Players    int      `json:"players"`
	Pool       []string `json:"pool"`
	Trials     int      `json:"trials"`
}

// ErrorResponse is returned when something goes wrong.
type ErrorResponse struct {
	Error string `json:"error"`
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func handleSimulate(rng *rand.Rand) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeJSON(w, http.StatusMethodNotAllowed, ErrorResponse{"method not allowed"})
			return
		}

		var req SimulateRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, ErrorResponse{"invalid JSON body"})
			return
		}

		if req.Players == 0 {
			req.Players = 5
		}
		if req.Trials == 0 {
			req.Trials = 100_000
		}
		for i := range req.Pool {
			req.Pool[i] = strings.TrimSpace(req.Pool[i])
		}
		if len(req.Pool) == 0 {
			writeJSON(w, http.StatusBadRequest, ErrorResponse{"pool must contain at least one item"})
			return
		}
		if req.TargetItem == "" {
			writeJSON(w, http.StatusBadRequest, ErrorResponse{"target_item is required"})
			return
		}

		sim := Simulation{
			Pool:       NewLootPool(req.Pool),
			Players:    req.Players,
			TargetItem: req.TargetItem,
		}

		result, err := sim.Run(req.Trials, rng)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, ErrorResponse{err.Error()})
			return
		}

		writeJSON(w, http.StatusOK, result)
	}
}

func handleDungeons(blizz *BlizzardClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeJSON(w, http.StatusMethodNotAllowed, ErrorResponse{"method not allowed"})
			return
		}

		dungeons, err := blizz.GetDungeons()
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, ErrorResponse{err.Error()})
			return
		}

		writeJSON(w, http.StatusOK, dungeons)
	}
}

func handleDungeonLoot(blizz *BlizzardClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeJSON(w, http.StatusMethodNotAllowed, ErrorResponse{"method not allowed"})
			return
		}

		// extract ID from /dungeons/{id}/loot
		parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
		if len(parts) < 3 {
			writeJSON(w, http.StatusBadRequest, ErrorResponse{"invalid path"})
			return
		}

		id, err := strconv.Atoi(parts[1])
		if err != nil {
			writeJSON(w, http.StatusBadRequest, ErrorResponse{"invalid dungeon ID"})
			return
		}

		items, err := blizz.GetLoot(id)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, ErrorResponse{err.Error()})
			return
		}

		writeJSON(w, http.StatusOK, items)
	}
}
