package scene

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// HeightMode is a global scene setting describing the viewer's vantage point over
// the ground plane. It widens (or not) the sense of perspective for the
// ground-plane elements — the base terrain, the ocean, and how far-off cities sit.
//
// The zero value is High, the original look (a photo from a high vantage point),
// so an older globals file that predates this field — and the v0 director, which
// never sets it — both mean "as before." Low is a near/at-ground-level vantage:
// the ground and ocean stretch much wider toward the viewer.
//
// It is derived from the seed by the scene.v1 director and read by the v1
// ground/cities/water renderers from the globals (so a recorded scene reproduces
// without re-deriving it). A scene's height can be overridden only by editing the
// recorded globals.
type HeightMode int

const (
	High HeightMode = iota
	Low
)

func (m HeightMode) String() string {
	switch m {
	case High:
		return "high"
	case Low:
		return "low"
	default:
		return "unknown"
	}
}

// MarshalYAML serializes a height as its lowercase name, so globals.yaml reads
// "high"/"low" rather than an opaque integer.
func (m HeightMode) MarshalYAML() (any, error) { return m.String(), nil }

// UnmarshalYAML parses a height from its name, rejecting unknown values so a
// malformed globals file fails loudly.
func (m *HeightMode) UnmarshalYAML(value *yaml.Node) error {
	var s string
	if err := value.Decode(&s); err != nil {
		return err
	}
	h, ok := ParseHeight(s)
	if !ok {
		return fmt.Errorf("scene: unknown height %q", s)
	}
	*m = h
	return nil
}

// ParseHeight parses a height-mode name. ok is false for unrecognized input
// (including the empty string), which callers treat as "use the default/zero
// value" (High).
func ParseHeight(s string) (m HeightMode, ok bool) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "high":
		return High, true
	case "low":
		return Low, true
	default:
		return High, false
	}
}
