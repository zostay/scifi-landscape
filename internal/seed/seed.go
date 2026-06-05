// Package seed turns a user-supplied seed string into an int64 scene seed.
//
// A seed may be given as a number or as arbitrary text. A plain base-10 integer
// that fits in an int64 is used directly, so numeric seeds round-trip exactly.
// Anything else — words, phrases, or numbers too large for an int64 — is hashed
// to a stable int64, so any string maps to one reproducible scene.
package seed

import (
	"hash/fnv"
	"strconv"
)

// Resolve converts a seed string to an int64.
func Resolve(s string) int64 {
	if n, err := strconv.ParseInt(s, 10, 64); err == nil {
		return n
	}
	h := fnv.New64a()
	h.Write([]byte(s))
	return int64(h.Sum64())
}

// IsNumeric reports whether s is a base-10 integer that fits in an int64, i.e.
// Resolve(s) uses it verbatim rather than hashing it.
func IsNumeric(s string) bool {
	_, err := strconv.ParseInt(s, 10, 64)
	return err == nil
}
