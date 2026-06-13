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
	elements   = map[string]Element{}
)

// RegisterElement registers el as the generator, renderer, and element for a
// versioned key. For v0 each scene element is its own generator and renderer, so
// this keeps the three registries in lockstep from one call. Call from an init
// function; it panics on a duplicate key.
func RegisterElement(key string, el Element) {
	RegisterGenerator(key, el)
	RegisterRenderer(key, el)
	if _, dup := elements[key]; dup {
		panic(fmt.Sprintf("scene: duplicate element key %q", key))
	}
	elements[key] = el
}

// ElementByName resolves an element key, or returns false if unregistered.
func ElementByName(key string) (Element, bool) {
	el, ok := elements[key]
	return el, ok
}

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
	// Each element is its own generator and renderer; register all three under one
	// versioned key. config.Algorithms names these keys to build a scene's pipeline.
	RegisterElement("sky.v0", &Sky{})
	RegisterElement("stars.v0", &Stars{})
	RegisterElement("systemstars.v0", &SystemStars{})
	RegisterElement("planets.v0", &Planets{})
	RegisterElement("clouds.v0", &Clouds{})
	RegisterElement("mountains.v0", &Mountains{})
	RegisterElement("ground.v0", &Ground{})
	RegisterElement("cities.v0", &Cities{})
	RegisterElement("water.v0", &Water{})
}
