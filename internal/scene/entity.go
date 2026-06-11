package scene

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

// An Entity is one component of a scene — visible (a planet, a ring) or
// informational. Each entity is an instance of a versioned schema: a Go struct
// whose name ends in V<n> for the 0-based schema version. Schemas are
// forward-mutable only — fields may be added, but existing fields are never
// renamed, retyped, or given new meaning. A change that would break an existing
// field is instead a brand-new schema (V<n+1>), so an entity recorded in an old
// scene file always deserializes and renders the same.
//
// The Generators produce entities from the globals; the Renderers consume them
// to draw the image. Because entities are plain serializable data, the scene
// list can be written to (and read back from) a scene file's YAML, letting a
// scene be reproduced from the entity layer alone.
type Entity interface {
	// EntitySchema is the entity's versioned schema key, e.g.
	// "planet.gasgiant.v0". It is the discriminator under which the entity is
	// registered and serialized.
	EntitySchema() string
}

// SceneList is the ordered list of entity instances that makes up a generated
// scene. Generators append to it; Renderers walk it in order.
type SceneList []Entity

// A Generator turns the globals into a list of entities for one part of the
// scene. It reads the Context's per-element random stream and settings but has no
// side effects — it draws nothing — so the same globals always produce the same
// scene list. (The Context bundles the globals together with the element's
// stream; future work may narrow this to a globals-only input.)
type Generator interface {
	Name() string
	Generate(c *Context) (SceneList, error)
}

// A Renderer draws a list of entities onto the canvas. It is the only thing
// permitted to modify the image, and it consumes no randomness, so a recorded
// scene list redraws to the same pixels every time. Renderers are versioned and
// frozen once released, like directors and generators.
type Renderer interface {
	Name() string
	RenderList(c *Context, list SceneList) error
}

// entityFactories maps each registered schema key to a constructor returning a
// fresh, zero-valued instance (a pointer, so YAML can populate it). It is the
// registry that lets a heterogeneous scene list round-trip through YAML: the
// schema key recorded next to each entity selects the type to decode into.
var entityFactories = map[string]func() Entity{}

// RegisterEntity registers an entity schema's constructor under its key. It
// panics on a duplicate key, since that means two schemas collide — a
// programming error caught at startup. Call it from an init function.
func RegisterEntity(schema string, factory func() Entity) {
	if _, dup := entityFactories[schema]; dup {
		panic(fmt.Sprintf("scene: duplicate entity schema %q", schema))
	}
	entityFactories[schema] = factory
}

// EntitySchemaRegistered reports whether a schema key has a registered factory.
func EntitySchemaRegistered(schema string) bool {
	_, ok := entityFactories[schema]
	return ok
}

// wireEntity is the on-disk shape of one scene-list entry: the schema key plus
// the entity's own fields under data. On decode, Data is deferred (yaml.Node) so
// the schema can pick the concrete type to decode into; on encode, Data carries
// the entity directly. Schema is listed first so it reads at the top of each
// entry.
type wireEntity struct {
	Schema string    `yaml:"schema"`
	Data   yaml.Node `yaml:"data"`
}

// MarshalYAML renders the scene list as a sequence of {schema, data} mappings, so
// a heterogeneous list serializes with each entity tagged by its schema (schema
// first for readability).
func (sl SceneList) MarshalYAML() (any, error) {
	type wireOut struct {
		Schema string `yaml:"schema"`
		Data   Entity `yaml:"data"`
	}
	out := make([]wireOut, len(sl))
	for i, e := range sl {
		out[i] = wireOut{Schema: e.EntitySchema(), Data: e}
	}
	return out, nil
}

// UnmarshalYAML reconstructs a scene list from {schema, data} mappings, decoding
// each entry's data into a fresh instance of the registered schema type. An
// unknown schema key is an error rather than a silently dropped entity, so a
// scene file produced by a newer version fails loudly instead of rendering wrong.
func (sl *SceneList) UnmarshalYAML(value *yaml.Node) error {
	var raw []wireEntity
	if err := value.Decode(&raw); err != nil {
		return fmt.Errorf("scene list: %w", err)
	}
	out := make(SceneList, 0, len(raw))
	for i, r := range raw {
		factory, ok := entityFactories[r.Schema]
		if !ok {
			return fmt.Errorf("scene list: entity %d: unknown schema %q", i, r.Schema)
		}
		e := factory()
		if err := r.Data.Decode(e); err != nil {
			return fmt.Errorf("scene list: entity %d (%s): %w", i, r.Schema, err)
		}
		out = append(out, e)
	}
	*sl = out
	return nil
}

// MarshalSceneList serializes a scene list to YAML for embedding in a scene file.
func MarshalSceneList(sl SceneList) ([]byte, error) {
	data, err := yaml.Marshal(sl)
	if err != nil {
		return nil, fmt.Errorf("scene list: %w", err)
	}
	return data, nil
}

// UnmarshalSceneList parses a scene list from YAML produced by MarshalSceneList.
func UnmarshalSceneList(data []byte) (SceneList, error) {
	var sl SceneList
	if err := yaml.Unmarshal(data, &sl); err != nil {
		return nil, err
	}
	return sl, nil
}
