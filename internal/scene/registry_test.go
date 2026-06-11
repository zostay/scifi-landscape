package scene

import (
	"regexp"
	"testing"

	"github.com/zostay/scifi-landscape/internal/config"
)

// versionedKey matches a frozen, versioned registry/schema key: a dotted name
// ending in v<n>, e.g. "planets.v0" or "planet.gasgiant.v0".
var versionedKey = regexp.MustCompile(`\.v\d+$`)

// TestEntitySchemasVersioned enforces the naming contract: every registered
// entity schema key ends in a version suffix, so a future change lands as a new
// V<n> rather than mutating an existing schema.
func TestEntitySchemasVersioned(t *testing.T) {
	if len(entityFactories) == 0 {
		t.Fatal("no entity schemas registered")
	}
	for key := range entityFactories {
		if !versionedKey.MatchString(key) {
			t.Errorf("entity schema %q is not versioned (must end in .v<n>)", key)
		}
		// The factory must produce an entity that reports the same schema key.
		if got := entityFactories[key]().EntitySchema(); got != key {
			t.Errorf("schema %q factory reports %q", key, got)
		}
	}
}

// TestAlgorithmKeysVersioned enforces the same contract for generators and
// renderers.
func TestAlgorithmKeysVersioned(t *testing.T) {
	for _, key := range GeneratorKeys() {
		if !versionedKey.MatchString(key) {
			t.Errorf("generator key %q is not versioned", key)
		}
	}
	for _, key := range RendererKeys() {
		if !versionedKey.MatchString(key) {
			t.Errorf("renderer key %q is not versioned", key)
		}
	}
}

// TestEveryRendererHasGenerator checks the pipeline-completeness contract: a
// renderer draws entities that something must generate, so every registered
// renderer key must have a matching generator.
func TestEveryRendererHasGenerator(t *testing.T) {
	for _, key := range RendererKeys() {
		if _, ok := GeneratorByName(key); !ok {
			t.Errorf("renderer %q has no matching generator", key)
		}
	}
}

// TestDefaultConfigDirectorResolves checks that the director named by the default
// config is actually registered, so a recorded config can be run.
func TestDefaultConfigDirectorResolves(t *testing.T) {
	dirs := config.DefaultConfig().Algorithms.Directors
	if len(dirs) == 0 {
		t.Fatal("default config names no director")
	}
	for _, name := range dirs {
		if _, ok := DirectorByName(name); !ok {
			t.Errorf("default config director %q is not registered", name)
		}
	}
}

// TestPlanetsRegistered checks the migrated proof element is wired into both
// registries and that its entity schemas exist.
func TestPlanetsRegistered(t *testing.T) {
	if _, ok := GeneratorByName("planets.v0"); !ok {
		t.Error("planets.v0 generator not registered")
	}
	if _, ok := RendererByName("planets.v0"); !ok {
		t.Error("planets.v0 renderer not registered")
	}
	if !EntitySchemaRegistered(SchemaPlanetGasGiantV0) || !EntitySchemaRegistered(SchemaPlanetMoonV0) {
		t.Error("planet entity schemas not registered")
	}
}
