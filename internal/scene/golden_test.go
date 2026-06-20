package scene

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/zostay/scifi-landscape/internal/canvas"
	"github.com/zostay/scifi-landscape/internal/config"
)

// The golden suite is the project's reproducibility safety net. Seed
// reproducibility is the sacred, non-negotiable invariant of this app: a given
// seed must always produce the exact same scene. As the configuration /
// directors / generators / renderers refactor proceeds, every step is a
// mechanical reorganization that must NOT change a single pixel for any seed in
// this matrix. This test renders that matrix headlessly, hashes the raw RGBA
// pixels, and compares against testdata/golden.txt.
//
// To (re)generate the goldens after an INTENTIONAL output change, run:
//
//	UPDATE_GOLDEN=1 go test ./internal/scene -run TestGolden
//
// Regenerating is a deliberate act: only do it when you mean to change what a
// seed looks like, and review the diff to golden.txt as part of that change.

// goldenCase is one entry in the reproducibility matrix.
type goldenCase struct {
	name string
	seed int64
	w, h int
	time string // time-of-day override; "" means derive from the seed
	// cfg, when non-nil, overrides the configuration (and thus the director and the
	// generator/renderer pipeline) for this case. It lets the matrix freeze pipelines
	// other than the default — the now-non-default v0 ground-plane algorithms, and the
	// v1 algorithms forced into their low/high branches. nil means the default config.
	cfg *config.Config
}

// goldenCases is chosen to exercise the breadth of the pipeline: a spread of
// seeds (different element presence/absence, ocean vs none, planet counts and
// sizes), all three times of day forced on a fixed seed, and a couple of large
// renders so the size-gated detail paths (moon craters, large-moon bump mapping)
// are covered too.
func goldenCases() []goldenCase {
	var cs []goldenCase
	// A spread of seeds at a modest size, natural (seed-derived) time of day.
	for _, s := range []int64{1, 2, 3, 7, 11, 42, 100, 256, 1024, 31337, 999983, -5} {
		cs = append(cs, goldenCase{
			name: fmt.Sprintf("s%d_480x270", s),
			seed: s, w: 480, h: 270,
		})
	}
	// All three times of day forced on a couple of fixed seeds, to cover every
	// time-of-day branch regardless of which the seeds happen to pick.
	for _, s := range []int64{42, 7} {
		for _, tod := range []string{"midday", "dusk", "twilight"} {
			cs = append(cs, goldenCase{
				name: fmt.Sprintf("s%d_480x270_%s", s, tod),
				seed: s, w: 480, h: 270, time: tod,
			})
		}
	}
	// Large renders so the size-gated detail paths run (big planets/moons).
	for _, s := range []int64{42, 256} {
		cs = append(cs, goldenCase{
			name: fmt.Sprintf("s%d_1280x720", s),
			seed: s, w: 1280, h: 720,
		})
	}
	// The default pipeline now selects the v1 ground-plane algorithms with a
	// seed-rolled height. These extra cases freeze the algorithms the default matrix
	// above does not pin on its own:
	//   - the now-non-default v0 ground/cities/water (old configs and scene files
	//     still select them, so their output must stay frozen), and
	//   - the v1 algorithms forced into each height branch (low and high), so both
	//     branches are covered regardless of which the default seeds happen to roll.
	v0 := allV0Config()
	low := forcedHeightConfig(1.0)  // every scene low
	high := forcedHeightConfig(0.0) // every scene high (must equal v0 output)
	for _, s := range []int64{1, 7, 42, 256} {
		cs = append(cs,
			goldenCase{name: fmt.Sprintf("v0_s%d_480x270", s), seed: s, w: 480, h: 270, cfg: &v0},
			goldenCase{name: fmt.Sprintf("low_s%d_480x270", s), seed: s, w: 480, h: 270, cfg: &low},
			goldenCase{name: fmt.Sprintf("high_s%d_480x270", s), seed: s, w: 480, h: 270, cfg: &high},
		)
	}
	return cs
}

// allV0Config returns the default config with its pipeline pinned to the original
// v0 director and v0 ground-plane algorithms, so the matrix freezes the algorithms
// old configs/scene files still select.
func allV0Config() config.Config {
	c := config.DefaultConfig()
	c.Algorithms.Directors = []string{"scene.v0"}
	v0 := []string{
		"sky.v0", "stars.v0", "systemstars.v0", "planets.v0",
		"clouds.v0", "mountains.v0", "ground.v0", "cities.v0", "water.v0",
	}
	c.Algorithms.Generators = append([]string(nil), v0...)
	c.Algorithms.Renderers = append([]string(nil), v0...)
	return c
}

// forcedHeightConfig returns the default (v1) config with the height roll pinned:
// lowChance 1.0 forces every scene to the low vantage, 0.0 forces high. It lets the
// matrix freeze both v1 branches deterministically.
func forcedHeightConfig(lowChance float64) config.Config {
	c := config.DefaultConfig()
	c.Perspective.LowChance = lowChance
	return c
}

// renderHash builds a scene headlessly (instant, no animation delay) and returns
// the hex SHA-256 of its raw RGBA pixels. Instant mode changes only timing, so
// the hash is identical to what an animated build of the same seed produces.
func renderHash(t *testing.T, c goldenCase) string {
	t.Helper()
	cfg := config.DefaultConfig()
	if c.cfg != nil {
		cfg = *c.cfg
	}
	// Resolve the director named by the config (the v0 cases select scene.v0),
	// falling back to the default, mirroring how the binaries build globals.
	dir := DefaultDirector()
	if dirs := cfg.Algorithms.Directors; len(dirs) > 0 {
		if d, ok := DirectorByName(dirs[0]); ok {
			dir = d
		}
	}
	globals := dir.Direct(cfg, c.seed, c.time, c.w, c.h)
	cv := canvas.New(c.w, c.h)
	sc, err := New(globals, cfg.Algorithms)
	if err != nil {
		t.Fatalf("%s: New: %v", c.name, err)
	}
	ctx := WithInstant(context.Background())
	if _, err := sc.Build(ctx, cv, c.seed, c.w, c.h, nil); err != nil {
		t.Fatalf("%s: build: %v", c.name, err)
	}
	buf := make([]byte, c.w*c.h*4)
	cv.Snapshot(buf)
	sum := sha256.Sum256(buf)
	return hex.EncodeToString(sum[:])
}

const goldenFile = "testdata/golden.txt"

func readGolden(t *testing.T) map[string]string {
	t.Helper()
	f, err := os.Open(goldenFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		t.Fatalf("open %s: %v", goldenFile, err)
	}
	defer f.Close()
	m := map[string]string{}
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		name, hash, ok := strings.Cut(line, " ")
		if !ok {
			t.Fatalf("malformed golden line: %q", line)
		}
		m[name] = strings.TrimSpace(hash)
	}
	if err := sc.Err(); err != nil {
		t.Fatalf("read %s: %v", goldenFile, err)
	}
	return m
}

func writeGolden(t *testing.T, hashes map[string]string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(goldenFile), 0o755); err != nil {
		t.Fatalf("mkdir testdata: %v", err)
	}
	names := make([]string, 0, len(hashes))
	for n := range hashes {
		names = append(names, n)
	}
	sort.Strings(names)
	var b strings.Builder
	b.WriteString("# scifi-landscape reproducibility goldens: <case> <sha256 of RGBA pixels>.\n")
	b.WriteString("# Regenerate intentionally with UPDATE_GOLDEN=1 go test ./internal/scene -run TestGolden\n")
	for _, n := range names {
		fmt.Fprintf(&b, "%s %s\n", n, hashes[n])
	}
	if err := os.WriteFile(goldenFile, []byte(b.String()), 0o644); err != nil {
		t.Fatalf("write %s: %v", goldenFile, err)
	}
}

// TestGolden is the reproducibility gate. It fails if any seed in the matrix
// renders differently than the recorded golden hash.
func TestGolden(t *testing.T) {
	cases := goldenCases()
	got := make(map[string]string, len(cases))
	for _, c := range cases {
		got[c.name] = renderHash(t, c)
	}

	if os.Getenv("UPDATE_GOLDEN") != "" {
		writeGolden(t, got)
		t.Logf("wrote %d goldens to %s", len(got), goldenFile)
		return
	}

	want := readGolden(t)
	if want == nil {
		t.Fatalf("no goldens recorded; run UPDATE_GOLDEN=1 go test ./internal/scene -run TestGolden")
	}
	for _, c := range cases {
		w, ok := want[c.name]
		if !ok {
			t.Errorf("%s: no golden recorded (run UPDATE_GOLDEN=1 to add)", c.name)
			continue
		}
		if got[c.name] != w {
			t.Errorf("%s: output changed\n  got  %s\n  want %s\nIf intentional, regenerate with UPDATE_GOLDEN=1.", c.name, got[c.name], w)
		}
	}
	for name := range want {
		if _, ok := got[name]; !ok {
			t.Errorf("golden %q has no matching case (stale entry?)", name)
		}
	}
}

// TestInstantMatchesAnimated guards the safety net itself: the instant
// (no-delay) build used by golden tests must produce byte-identical pixels to a
// normally animated build. If it ever diverged, the goldens would be measuring
// something the live app never produces. It renders one seed both ways and
// compares the pixels. Skipped in -short mode because the animated path sleeps.
func TestInstantMatchesAnimated(t *testing.T) {
	if testing.Short() {
		t.Skip("animated build sleeps; skipped in -short")
	}
	const w, h = 200, 120
	const sd = 42

	cfg := config.DefaultConfig()
	build := func(ctx context.Context) [32]byte {
		cv := canvas.New(w, h)
		sc, err := New(DefaultDirector().Direct(cfg, sd, "", w, h), cfg.Algorithms)
		if err != nil {
			t.Fatalf("New: %v", err)
		}
		if _, err := sc.Build(ctx, cv, sd, w, h, nil); err != nil {
			t.Fatalf("build: %v", err)
		}
		buf := make([]byte, w*h*4)
		cv.Snapshot(buf)
		return sha256.Sum256(buf)
	}

	instant := build(WithInstant(context.Background()))
	animated := build(context.Background())
	if instant != animated {
		t.Fatalf("instant build differs from animated build:\n  instant  %x\n  animated %x", instant, animated)
	}
}
