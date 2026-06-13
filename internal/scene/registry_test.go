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

// TestDefaultConfigPipelineResolves checks that every generator and renderer key
// the default config names is actually registered — so the config that scene
// files record really does build the pipeline. (This guards against the keys
// drifting out of lockstep with the registry, e.g. bare "sky" vs "sky.v0".)
func TestDefaultConfigPipelineResolves(t *testing.T) {
	algos := config.DefaultConfig().Algorithms
	if len(algos.Generators) == 0 || len(algos.Renderers) == 0 {
		t.Fatal("default config names no generators/renderers")
	}
	if err := CheckAlgorithms(algos); err != nil {
		t.Fatalf("default config pipeline does not resolve: %v", err)
	}
}

// TestNewResolvesPipelineFromConfig checks New builds the pipeline named by the
// config (in order) and fails loudly on an unknown algorithm key.
func TestNewResolvesPipelineFromConfig(t *testing.T) {
	g := globalsFor(1, "", 480, 270)

	// A subset, in a non-default order, is honored exactly.
	sc, err := New(g, config.Algorithms{
		Generators: []string{"ground.v0", "sky.v0"},
		Renderers:  []string{"ground.v0", "sky.v0"},
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if len(sc.Elements) != 2 || sc.Elements[0].Name() != "ground" || sc.Elements[1].Name() != "sky" {
		t.Errorf("pipeline = %v, want [ground sky]", elementNames(sc.Elements))
	}

	// An unknown generator key is an error.
	if _, err := New(g, config.Algorithms{Generators: []string{"nope.v0"}}); err == nil {
		t.Error("New: want error for unknown generator key, got nil")
	}
	// An unknown renderer key is an error too.
	if err := CheckAlgorithms(config.Algorithms{Renderers: []string{"nope.v0"}}); err == nil {
		t.Error("CheckAlgorithms: want error for unknown renderer key, got nil")
	}
}

func elementNames(els []Element) []string {
	out := make([]string, len(els))
	for i, el := range els {
		out[i] = el.Name()
	}
	return out
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
