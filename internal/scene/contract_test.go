package scene

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/zostay/scifi-landscape/internal/config"
)

// The contract tests enforce the release change-contract (see VERSIONING.md)
// mechanically, alongside the golden suite:
//
//   - TestGolden (golden_test.go) is the BEHAVIORAL freeze: a released algorithm
//     must not change the pixels it produces.
//   - TestSchemaContract (here) is the STRUCTURAL freeze: the serialized data
//     shapes (entity schemas, globals, config) may only grow — an existing
//     serialized field is never renamed, retyped, or removed.
//   - TestGoldenCoversAllAlgorithms (here) guards that every registered algorithm
//     is actually exercised by the golden suite, so nothing escapes the behavioral
//     freeze.
//
// Run them any time with `go test ./internal/scene` (or `make verify`); the
// `/release` skill runs the whole gate before tagging.

// modulePrefix bounds recursion to this project's own types: we record the type
// of every serialized field, but only descend into structs we own (so external
// types like yaml.Node are noted, not expanded).
const modulePrefix = "github.com/zostay/scifi-landscape/"

const contractSchemaFile = "testdata/contract-schema.txt"

// yamlKey returns the on-disk YAML key for a struct field and whether it is
// serialized at all, matching gopkg.in/yaml.v3: an explicit `yaml:"name"` tag
// wins; `yaml:"-"` (or an unexported field) is skipped; otherwise the key is the
// lowercased field name.
func yamlKey(f reflect.StructField) (string, bool) {
	if f.PkgPath != "" { // unexported: never serialized
		return "", false
	}
	tag := f.Tag.Get("yaml")
	if tag == "-" {
		return "", false
	}
	name := tag
	if i := strings.IndexByte(tag, ','); i >= 0 {
		name = tag[:i]
	}
	if name == "" { // no tag, or `yaml:",omitempty"`
		name = strings.ToLower(f.Name)
	}
	if name == "-" {
		return "", false
	}
	return name, true
}

// recordType walks t, recording one `Type/yamlKey goType` line per serialized
// field of every struct it owns that t reaches (through pointers, slices, arrays,
// and maps). out is a set so traversal order does not matter; visited breaks
// cycles and avoids rewalking shared types.
func recordType(t reflect.Type, out map[string]bool, visited map[reflect.Type]bool) {
	switch t.Kind() {
	case reflect.Pointer, reflect.Slice, reflect.Array:
		recordType(t.Elem(), out, visited)
	case reflect.Map:
		recordType(t.Key(), out, visited)
		recordType(t.Elem(), out, visited)
	case reflect.Struct:
		if !strings.HasPrefix(t.PkgPath(), modulePrefix) || visited[t] {
			return // external type, or already walked
		}
		visited[t] = true
		for i := 0; i < t.NumField(); i++ {
			f := t.Field(i)
			key, ok := yamlKey(f)
			if !ok {
				continue
			}
			out[fmt.Sprintf("%s/%s %s", t.String(), key, f.Type.String())] = true
			recordType(f.Type, out, visited)
		}
	}
}

// schemaSignature is the sorted structural signature of everything the system
// serializes: every registered entity schema, the globals, and the config.
func schemaSignature() []string {
	out := map[string]bool{}
	visited := map[reflect.Type]bool{}
	for _, factory := range entityFactories {
		recordType(reflect.TypeOf(factory()), out, visited)
	}
	recordType(reflect.TypeOf(Globals{}), out, visited)
	recordType(reflect.TypeOf(config.Config{}), out, visited)

	lines := make([]string, 0, len(out))
	for l := range out {
		lines = append(lines, l)
	}
	sort.Strings(lines)
	return lines
}

// TestSchemaContract is the structural freeze: every serialized field recorded in
// the baseline must still exist with the same YAML key and Go type. A missing
// line means a released field was renamed, retyped, or removed (or a whole schema
// was dropped) — forbidden. New fields/schemas are allowed (they only add lines).
// Regenerate the baseline (after an intentional, additive change) with:
//
//	UPDATE_CONTRACT=1 go test ./internal/scene -run TestSchemaContract
func TestSchemaContract(t *testing.T) {
	cur := schemaSignature()

	if os.Getenv("UPDATE_CONTRACT") != "" {
		writeContract(t, cur)
		t.Logf("wrote %d schema-contract lines to %s", len(cur), contractSchemaFile)
		return
	}

	base := readContract(t)
	if base == nil {
		t.Fatalf("no schema contract recorded; run UPDATE_CONTRACT=1 go test ./internal/scene -run TestSchemaContract")
	}
	have := make(map[string]bool, len(cur))
	for _, l := range cur {
		have[l] = true
	}
	for _, l := range base {
		if !have[l] {
			t.Errorf("schema contract violated: %q is gone.\nA released serialized field must not be renamed, retyped, or removed — add a new versioned schema/field instead (PlanetGasGiantV0 -> ...V1). If this is a deliberate, additive new baseline, regenerate with UPDATE_CONTRACT=1.", l)
		}
	}
}

// TestGoldenCoversAllAlgorithms guards the behavioral freeze's completeness: every
// registered generator and renderer must appear in the default config's pipeline,
// which the golden suite renders — so no released algorithm can change unnoticed.
//
// When a new version is added outside the default pipeline (e.g. sky.v1, while the
// default stays sky.v0), this fails on purpose: add a golden case whose config
// selects the new algorithm so its output is frozen, then this passes again.
func TestGoldenCoversAllAlgorithms(t *testing.T) {
	algos := config.DefaultConfig().Algorithms
	gens := toSet(algos.Generators)
	rends := toSet(algos.Renderers)

	for _, k := range GeneratorKeys() {
		if !gens[k] {
			t.Errorf("generator %q is registered but not in the default (golden-covered) pipeline; the golden suite does not freeze its behavior. Add a golden case whose config selects it before release.", k)
		}
	}
	for _, k := range RendererKeys() {
		if !rends[k] {
			t.Errorf("renderer %q is registered but not in the default (golden-covered) pipeline; add a golden case whose config selects it before release.", k)
		}
	}
	for _, d := range algos.Directors {
		if _, ok := DirectorByName(d); !ok {
			t.Errorf("default config director %q is not registered", d)
		}
	}
}

func toSet(ss []string) map[string]bool {
	m := make(map[string]bool, len(ss))
	for _, s := range ss {
		m[s] = true
	}
	return m
}

func readContract(t *testing.T) []string {
	t.Helper()
	f, err := os.Open(contractSchemaFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		t.Fatalf("open %s: %v", contractSchemaFile, err)
	}
	defer f.Close()
	var lines []string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		lines = append(lines, line)
	}
	if err := sc.Err(); err != nil {
		t.Fatalf("read %s: %v", contractSchemaFile, err)
	}
	return lines
}

func writeContract(t *testing.T, lines []string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(contractSchemaFile), 0o755); err != nil {
		t.Fatalf("mkdir testdata: %v", err)
	}
	var b strings.Builder
	b.WriteString("# scifi-landscape schema contract: one `<Type>/<yamlKey> <goType>` line per\n")
	b.WriteString("# serialized field. Frozen: lines may be ADDED but never removed/changed.\n")
	b.WriteString("# Regenerate after an additive change with:\n")
	b.WriteString("#   UPDATE_CONTRACT=1 go test ./internal/scene -run TestSchemaContract\n")
	for _, l := range lines {
		fmt.Fprintln(&b, l)
	}
	if err := os.WriteFile(contractSchemaFile, []byte(b.String()), 0o644); err != nil {
		t.Fatalf("write %s: %v", contractSchemaFile, err)
	}
}
