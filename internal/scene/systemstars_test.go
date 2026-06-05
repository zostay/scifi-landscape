package scene

import (
	"math/rand"
	"testing"
)

func TestSystemStarCountInRange(t *testing.T) {
	rng := rand.New(rand.NewSource(99))
	for range 5000 {
		for _, tod := range []TimeOfDay{Midday, Dusk} {
			if n := systemStarCount(rng, tod); n < 0 || n > sysMaxCount {
				t.Fatalf("count %d out of [0,%d] for %v", n, sysMaxCount, tod)
			}
		}
	}
}

// Dusk should yield fewer suns overall than midday (the spec wants higher
// counts much less likely at dusk).
func TestSystemStarsDuskFewerThanMidday(t *testing.T) {
	const n = 20000
	midday, dusk := 0, 0
	rm := rand.New(rand.NewSource(1))
	rd := rand.New(rand.NewSource(1))
	for range n {
		midday += systemStarCount(rm, Midday)
		dusk += systemStarCount(rd, Dusk)
	}
	if dusk >= midday {
		t.Errorf("expected fewer suns at dusk: midday total %d, dusk total %d", midday, dusk)
	}
}

func TestSystemStarsCommonlyZeroOrOne(t *testing.T) {
	const n = 20000
	rng := rand.New(rand.NewSource(7))
	zeroOrOne := 0
	for range n {
		if c := systemStarCount(rng, Midday); c <= 1 {
			zeroOrOne++
		}
	}
	if frac := float64(zeroOrOne) / n; frac < 0.70 {
		t.Errorf("0-or-1 suns only %.0f%% of the time, want >=70%%", frac*100)
	}
}
