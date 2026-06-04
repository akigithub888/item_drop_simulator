package main

import (
	"fmt"
	"math"
	"math/rand"
)

// Item represents a single item in the loot pool.
type Item struct {
	Name string
}

// LootPool holds a collection of items with equal drop chances.
type LootPool struct {
	Items []Item
}

// Simulation holds the configuration for a single drop simulation.
type Simulation struct {
	Pool       LootPool
	Players    int
	Traders    int // group members who can also get the item and trade it to you
	TargetItem string
}

// SimulationResult holds the outcome of a simulation.
type SimulationResult struct {
	TargetItem    string       `json:"target_item"`
	DropChance    float64      `json:"drop_chance"`
	ExpectedRuns  float64      `json:"expected_runs"`
	SimulatedRuns float64      `json:"simulated_runs"`
	Trials        int          `json:"trials"`
	Curve         []CurvePoint `json:"curve"`
}

// CurvePoint represents the number of runs needed to reach a probability threshold.
type CurvePoint struct {
	Probability float64 `json:"probability"`
	Runs        int     `json:"runs"`
}

// NewLootPool creates a LootPool from a slice of item names.
func NewLootPool(names []string) LootPool {
	items := make([]Item, len(names))
	for i, name := range names {
		items[i] = Item{Name: name}
	}
	return LootPool{Items: items}
}

// Contains checks whether the pool has an item with the given name.
func (lp LootPool) Contains(name string) bool {
	for _, item := range lp.Items {
		if item.Name == name {
			return true
		}
	}
	return false
}

// Size returns the number of items in the pool.
func (lp LootPool) Size() int {
	return len(lp.Items)
}

// baseChance returns the single-player chance of getting the target item in one run.
// This is 1 / poolSize / players.
func (s Simulation) baseChance() float64 {
	return 1.0 / float64(s.Pool.Size()) / float64(s.Players)
}

// DropChance returns the probability of obtaining the target item in a single run,
// accounting for traders. Each trader has the same base chance to get it and
// trade it to you, so the combined chance is:
//
//	1 - (1 - baseChance)^(1 + traders)
func (s Simulation) DropChance() float64 {
	p := s.baseChance()
	return 1.0 - math.Pow(1-p, float64(1+s.Traders))
}

// ExpectedRuns returns the average number of runs needed (geometric distribution).
func (s Simulation) ExpectedRuns() float64 {
	return 1.0 / s.DropChance()
}

// RunsForProbability returns how many runs are needed to reach a given probability threshold.
func (s Simulation) RunsForProbability(target float64) int {
	p := s.DropChance()
	if p >= 1.0 {
		return 1
	}
	return int(math.Ceil(math.Log(1-target) / math.Log(1-p)))
}

// buildCurve generates curve points at standard probability thresholds.
func (s Simulation) buildCurve() []CurvePoint {
	thresholds := []float64{0.25, 0.50, 0.75, 0.90, 0.99}
	curve := make([]CurvePoint, len(thresholds))
	for i, t := range thresholds {
		curve[i] = CurvePoint{Probability: t, Runs: s.RunsForProbability(t)}
	}
	return curve
}

// simulateRun runs one dungeon and returns true if you or any trader got the item.
// You are player 0; traders are players 1..Traders.
// Everyone rolls independently — each person gets a random item from the pool,
// and the item goes to a random player in the group.
func (s Simulation) simulateRun(rng *rand.Rand) bool {
	eligible := 1 + s.Traders // you + traders
	for i := 0; i < eligible; i++ {
		droppedItem := s.Pool.Items[rng.Intn(s.Pool.Size())]
		luckyPlayer := rng.Intn(s.Players)
		// luckyPlayer == 0 means the item goes to this eligible person (you or a trader
		// who would then trade it to you). We model each trader as "player 0" of their
		// own independent roll since we assume same pool and same group size.
		if droppedItem.Name == s.TargetItem && luckyPlayer == 0 {
			return true
		}
	}
	return false
}

// Run executes the simulation for a given number of trials and returns a result.
func (s Simulation) Run(trials int, rng *rand.Rand) (SimulationResult, error) {
	if !s.Pool.Contains(s.TargetItem) {
		return SimulationResult{}, fmt.Errorf("item %q not found in loot pool", s.TargetItem)
	}
	if s.Players <= 0 {
		return SimulationResult{}, fmt.Errorf("players must be greater than 0")
	}
	if s.Pool.Size() == 0 {
		return SimulationResult{}, fmt.Errorf("loot pool is empty")
	}
	if s.Traders < 0 {
		return SimulationResult{}, fmt.Errorf("traders cannot be negative")
	}
	if s.Traders > s.Players-1 {
		return SimulationResult{}, fmt.Errorf("traders cannot exceed players-1 (%d)", s.Players-1)
	}

	totalRuns := 0
	for i := 0; i < trials; i++ {
		runs := 0
		for {
			runs++
			if s.simulateRun(rng) {
				break
			}
		}
		totalRuns += runs
	}

	return SimulationResult{
		TargetItem:    s.TargetItem,
		DropChance:    s.DropChance(),
		ExpectedRuns:  s.ExpectedRuns(),
		SimulatedRuns: float64(totalRuns) / float64(trials),
		Trials:        trials,
		Curve:         s.buildCurve(),
	}, nil
}
