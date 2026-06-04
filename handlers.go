package main

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"strconv"
	"strings"
)

type SimulateRequest struct {
	TargetItem string   `json:"target_item"`
	Players    int      `json:"players"`
	Traders    int      `json:"traders"`
	Pool       []string `json:"pool"`
	Trials     int      `json:"trials"`
}

type ErrorResponse struct {
	Error string `json:"error"`
}

type ParseSimCResponse struct {
	Class    string `json:"class"`
	Spec     string `json:"spec"`
	SpecID   int    `json:"spec_id"`
	ClassID  int    `json:"class_id"`
	SpecName string `json:"spec_name"`
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
			Traders:    req.Traders,
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

func handleDungeonLoot(loot *LootData) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeJSON(w, http.StatusMethodNotAllowed, ErrorResponse{"method not allowed"})
			return
		}

		parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
		if len(parts) < 3 {
			writeJSON(w, http.StatusBadRequest, ErrorResponse{"invalid path"})
			return
		}

		blizzardID, err := strconv.Atoi(parts[1])
		if err != nil {
			writeJSON(w, http.StatusBadRequest, ErrorResponse{"invalid dungeon ID"})
			return
		}

		luaID, ok := BlizzardToLuaInstanceID[blizzardID]
		if !ok {
			writeJSON(w, http.StatusBadRequest, ErrorResponse{fmt.Sprintf("dungeon %d is not in the current season", blizzardID)})
			return
		}

		specID := 0
		if s := r.URL.Query().Get("spec"); s != "" {
			specID, _ = strconv.Atoi(s)
		}

		items, err := loot.GetLootForSpec(luaID, specID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, ErrorResponse{err.Error()})
			return
		}

		writeJSON(w, http.StatusOK, items)
	}
}

func handleParseSimC(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, ErrorResponse{"method not allowed"})
		return
	}

	var body struct {
		SimC string `json:"simc"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{"invalid JSON body"})
		return
	}

	className, specName, info, ok := ParseSimC(body.SimC)
	if !ok {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{
			fmt.Sprintf("could not identify spec for class=%q spec=%q", className, specName),
		})
		return
	}

	writeJSON(w, http.StatusOK, ParseSimCResponse{
		Class:    className,
		Spec:     specName,
		SpecID:   info.SpecID,
		ClassID:  info.ClassID,
		SpecName: info.Name,
	})
}
