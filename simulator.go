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
	TargetItem string
}

type SimulationResult struct {
	TargetItem    string       `json:"target_item"`
	DropChance    float64      `json:"drop_chance"`
	ExpectedRuns  float64      `json:"expected_runs"`
	SimulatedRuns float64      `json:"simulated_runs"`
	Trials        int          `json:"trials"`
	Curve         []CurvePoint `json:"curve"`
}

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

// DropChance returns the probability of getting the target item in a single run.
func (s Simulation) DropChance() float64 {
	return 1.0 / float64(s.Pool.Size()) / float64(s.Players)
}

// ExpectedRuns returns the average number of runs needed (geometric distribution).
func (s Simulation) ExpectedRuns() float64 {
	return 1.0 / s.DropChance()
}

// RunsForProbability returns how many runs are needed to reach a given probability threshold.
func (s Simulation) RunsForProbability(target float64) int {
	p := s.DropChance()
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

// simulate runs one dungeon and returns true if the player got the target item.
func (s Simulation) simulate(rng *rand.Rand) bool {
	droppedItem := s.Pool.Items[rng.Intn(s.Pool.Size())]
	luckyPlayer := rng.Intn(s.Players)
	return droppedItem.Name == s.TargetItem && luckyPlayer == 0
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

	totalRuns := 0
	for i := 0; i < trials; i++ {
		runs := 0
		for {
			runs++
			if s.simulate(rng) {
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
