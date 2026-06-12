package scene

import "fmt"

// This file holds the algorithm registries that, together with the entity
// registry (entity.go) and the director registry (director.go), let a
// configuration name the algorithms that build a scene by versioned key. A
// configuration's Algorithms section lists director, generator, and renderer keys;
// resolving them here turns a recorded config into a runnable pipeline.
//
// Registry keys are versioned (e.g. "planets.v0"). They are distinct from an
// element's Name() — which is also its random-stream key and must never change
// for reproducibility — so an algorithm can gain a "planets.v1" without touching
// the "planets" stream key that pins existing seeds.

var (
	generators = map[string]Generator{}
	renderers  = map[string]Renderer{}
)

// RegisterGenerator registers a generator under a versioned key. It panics on a
// duplicate key (a startup-time programming error). Call from an init function.
func RegisterGenerator(key string, g Generator) {
	if _, dup := generators[key]; dup {
		panic(fmt.Sprintf("scene: duplicate generator key %q", key))
	}
	generators[key] = g
}

// RegisterRenderer registers a renderer under a versioned key. It panics on a
// duplicate key. Call from an init function.
func RegisterRenderer(key string, r Renderer) {
	if _, dup := renderers[key]; dup {
		panic(fmt.Sprintf("scene: duplicate renderer key %q", key))
	}
	renderers[key] = r
}

// GeneratorByName resolves a generator key, or returns false if unregistered.
func GeneratorByName(key string) (Generator, bool) {
	g, ok := generators[key]
	return g, ok
}

// RendererByName resolves a renderer key, or returns false if unregistered.
func RendererByName(key string) (Renderer, bool) {
	r, ok := renderers[key]
	return r, ok
}

// GeneratorKeys returns the registered generator keys (unordered).
func GeneratorKeys() []string { return keysOf(generators) }

// RendererKeys returns the registered renderer keys (unordered).
func RendererKeys() []string {
	out := make([]string, 0, len(renderers))
	for k := range renderers {
		out = append(out, k)
	}
	return out
}

func keysOf[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

func init() {
	// Planets is the first element migrated to the generator/renderer split; it
	// serves as both. As more elements migrate, register them here under their
	// own versioned keys.
	p := &Planets{}
	RegisterGenerator("planets.v0", p)
	RegisterRenderer("planets.v0", p)

	st := &Stars{}
	RegisterGenerator("stars.v0", st)
	RegisterRenderer("stars.v0", st)

	ss := &SystemStars{}
	RegisterGenerator("systemstars.v0", ss)
	RegisterRenderer("systemstars.v0", ss)

	mt := &Mountains{}
	RegisterGenerator("mountains.v0", mt)
	RegisterRenderer("mountains.v0", mt)
}
