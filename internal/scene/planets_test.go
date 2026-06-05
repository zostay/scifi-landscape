package scene

import (
	"math/rand"
	"testing"
)

func TestPlanetCountRange(t *testing.T) {
	rng := rand.New(rand.NewSource(11))
	for range 100000 {
		if n := planetCount(rng); n < 0 || n > planetMax {
			t.Fatalf("planetCount = %d, out of [0,%d]", n, planetMax)
		}
	}
}

// About a quarter of scenes should have no planets (planetChance = 0.75, i.e.
// the old 50% empty halved), and when planets are present multiples are common.
func TestPlanetCountDistribution(t *testing.T) {
	const n = 200000
	rng := rand.New(rand.NewSource(3))
	var empty, present, multipleWhenPresent int
	for range n {
		c := planetCount(rng)
		if c == 0 {
			empty++
			continue
		}
		present++
		if c >= 2 {
			multipleWhenPresent++
		}
	}

	if frac := float64(empty) / n; frac < 0.22 || frac > 0.28 {
		t.Errorf("empty fraction %.2f, want ~0.25", frac)
	}
	if frac := float64(multipleWhenPresent) / float64(present); frac < 0.55 {
		t.Errorf("only %.0f%% of present scenes have multiple planets, want >=55%%", frac*100)
	}
}
