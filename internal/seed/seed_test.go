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
