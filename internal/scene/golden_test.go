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
	return cs
}

// renderHash builds a scene headlessly (instant, no animation delay) and returns
// the hex SHA-256 of its raw RGBA pixels. Instant mode changes only timing, so
// the hash is identical to what an animated build of the same seed produces.
func renderHash(t *testing.T, c goldenCase) string {
	t.Helper()
	settings := NewSettings(c.seed, c.time, c.h)
	cv := canvas.New(c.w, c.h)
	sc := New(settings)
	ctx := WithInstant(context.Background())
	if err := sc.Build(ctx, cv, c.seed, c.w, c.h, nil); err != nil {
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

	build := func(ctx context.Context) [32]byte {
		cv := canvas.New(w, h)
		sc := New(NewSettings(sd, "", h))
		if err := sc.Build(ctx, cv, sd, w, h, nil); err != nil {
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
