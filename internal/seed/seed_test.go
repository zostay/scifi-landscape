package seed

import "testing"

func TestResolveNumericUsedDirectly(t *testing.T) {
	cases := []struct {
		in   string
		want int64
	}{
		{"0", 0},
		{"12345", 12345},
		{"-42", -42},
		{"9223372036854775807", 9223372036854775807}, // max int64
	}
	for _, c := range cases {
		if got := Resolve(c.in); got != c.want {
			t.Errorf("Resolve(%q) = %d, want %d", c.in, got, c.want)
		}
		if !IsNumeric(c.in) {
			t.Errorf("IsNumeric(%q) = false, want true", c.in)
		}
	}
}

func TestResolveTextIsHashedAndStable(t *testing.T) {
	if IsNumeric("mars") {
		t.Fatal(`IsNumeric("mars") = true, want false`)
	}
	a := Resolve("mars")
	b := Resolve("mars")
	if a != b {
		t.Errorf("Resolve is not stable: %d vs %d", a, b)
	}
	if Resolve("mars") == Resolve("venus") {
		t.Error("different strings hashed to the same seed")
	}
}

// Numbers too large for an int64 are not "numeric" and fall back to hashing,
// so they still produce a usable, stable seed instead of failing.
func TestResolveOverflowFallsBackToHash(t *testing.T) {
	const tooBig = "99999999999999999999999999"
	if IsNumeric(tooBig) {
		t.Fatalf("IsNumeric(%q) = true, want false (overflows int64)", tooBig)
	}
	if a, b := Resolve(tooBig), Resolve(tooBig); a != b {
		t.Errorf("overflow seed is not stable: %d vs %d", a, b)
	}
}

// Derive must be stable for a given (base, key), and independent across keys and
// across bases — that independence is what keeps each element's random stream
// from shifting when another element changes.
func TestDeriveStableAndIndependent(t *testing.T) {
	if a, b := Derive(42, "planets"), Derive(42, "planets"); a != b {
		t.Errorf("Derive is not stable for the same base and key: %d vs %d", a, b)
	}
	if Derive(42, "planets") == Derive(42, "clouds") {
		t.Error("different keys derived the same seed")
	}
	if Derive(42, "planets") == Derive(43, "planets") {
		t.Error("different bases derived the same seed")
	}

	// Distinct (base, key) pairs should rarely collide; over a decent sweep of the
	// keys a scene actually uses, expect none.
	keys := []string{"settings", "sky-gradient", "ground-gradient", "sky", "stars",
		"systemstars", "planets", "clouds", "mountains", "ground", "cities", "water"}
	seen := make(map[int64]string)
	for base := range int64(500) {
		for _, k := range keys {
			d := Derive(base, k)
			if prev, ok := seen[d]; ok {
				t.Fatalf("collision: base %d key %q vs %q", base, k, prev)
			}
			seen[d] = k
		}
	}
}
