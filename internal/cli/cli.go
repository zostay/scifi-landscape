// Package cli resolves the scene inputs shared by the windowed app and the
// headless renderer: the configuration and the starting seed, drawn from an
// optional config file and/or an existing scene file.
package cli

import (
	"fmt"
	"os"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/zostay/scifi-landscape/internal/config"
	"github.com/zostay/scifi-landscape/internal/scene"
	"github.com/zostay/scifi-landscape/internal/scenefile"
)

// SceneFlags holds the scene inputs shared by the windowed app and the headless
// renderer. Bind them to a cobra command with AddSceneFlags, then pass Seed,
// Config, and From to Resolve.
type SceneFlags struct {
	Seed   string
	Time   string
	Width  int
	Height int
	Config string
	From   string
}

// AddSceneFlags registers the common scene flags on cmd in POSIX -s/--long
// style and returns the struct they bind to. Note: -h is reserved by cobra for
// --help, so --height has no short form.
func AddSceneFlags(cmd *cobra.Command) *SceneFlags {
	f := &SceneFlags{}
	fl := cmd.Flags()
	fl.StringVarP(&f.Seed, "seed", "s", "", "scene seed: a number, or any text (hashed); empty picks a random one")
	fl.StringVarP(&f.Time, "time", "t", "", "force time of day: midday, dusk, or twilight")
	fl.IntVarP(&f.Width, "width", "w", 1280, "scene width in pixels")
	fl.IntVar(&f.Height, "height", 720, "scene height in pixels")
	fl.StringVarP(&f.Config, "config", "c", "", "YAML config file (partial or complete) to tune generation")
	fl.StringVarP(&f.From, "from", "f", "", "reproduce from a scene file (PNG): supplies seed and config unless overridden")
	return f
}

// sceneLayers enumerates the reproducibility layers a scene file can carry, in
// canonical order, pairing each selectable name with its scene-file text-chunk
// key and the filename suffix used when extracting it.
var sceneLayers = []struct {
	Name   string // selectable layer name: seed, config, globals, scene
	Key    string // scenefile text-chunk key
	Suffix string // output filename suffix, e.g. "config.yaml"
}{
	{"seed", scenefile.KeySeed, "seed.txt"},
	{"config", scenefile.KeyConfig, "config.yaml"},
	{"globals", scenefile.KeyGlobals, "globals.yaml"},
	{"scene", scenefile.KeySceneList, "scene.yaml"},
}

// ExtractedLayer is one reproducibility layer pulled from a scene file: the
// suggested output filename and the chunk content to write, verbatim.
type ExtractedLayer struct {
	Name    string // layer name: seed, config, globals, scene
	File    string // output filename: scifi-<seed>.<suffix>
	Content string // chunk content, exactly as embedded
}

// ExtractScene reads the scene file (PNG) at path and returns the requested
// layers as files named scifi-<seed>.<suffix>. want selects layers by name
// (seed, config, globals, scene); a layer that is requested but not embedded in
// the file is reported in missing instead of returned, so the caller can warn.
// The embedded seed names every output, so its absence is an error, as is a path
// that is not a valid PNG.
func ExtractScene(path string, want map[string]bool) (files []ExtractedLayer, missing []string, err error) {
	texts, err := scenefile.ReadTextsFile(path)
	if err != nil {
		return nil, nil, fmt.Errorf("read scene file %q: %w", path, err)
	}
	seedStr, ok := texts[scenefile.KeySeed]
	if !ok {
		return nil, nil, fmt.Errorf("scene file %q has no embedded seed", path)
	}
	for _, l := range sceneLayers {
		if !want[l.Name] {
			continue
		}
		content, ok := texts[l.Key]
		if !ok {
			missing = append(missing, l.Name)
			continue
		}
		files = append(files, ExtractedLayer{
			Name:    l.Name,
			File:    fmt.Sprintf("scifi-%s.%s", seedStr, l.Suffix),
			Content: content,
		})
	}
	return files, missing, nil
}

// Resolve builds the scene configuration and the starting seed string from the
// command-line inputs:
//
//   - fromPath, if set, is a scene file (PNG) whose embedded config and seed are
//     used as the baseline.
//   - configPath, if set, is a YAML config file (partial or complete) that is
//     completed from defaults; it takes precedence over a scene file's config.
//   - seedFlag, if set, is the user's explicit seed and takes precedence over a
//     scene file's seed.
//
// The returned seedStr is suitable for internal/seed.Resolve; it is empty only
// when no seed source was given, which the caller treats as "pick a random seed".
func Resolve(configPath, fromPath, seedFlag string) (cfg config.Config, seedStr string, err error) {
	cfg = config.DefaultConfig()
	var fileSeed string

	if fromPath != "" {
		texts, err := scenefile.ReadTextsFile(fromPath)
		if err != nil {
			return cfg, "", fmt.Errorf("read scene file %q: %w", fromPath, err)
		}
		ls, err := scene.LoadSceneTexts(texts)
		if err != nil {
			return cfg, "", fmt.Errorf("parse scene file %q: %w", fromPath, err)
		}
		cfg = ls.Config
		if ls.HasSeed {
			fileSeed = strconv.FormatInt(ls.Seed, 10)
		}
	}

	if configPath != "" {
		data, err := os.ReadFile(configPath)
		if err != nil {
			return cfg, "", fmt.Errorf("read config %q: %w", configPath, err)
		}
		cfg, err = config.Load(data)
		if err != nil {
			return cfg, "", fmt.Errorf("load config %q: %w", configPath, err)
		}
	}

	seedStr = seedFlag
	if seedStr == "" {
		seedStr = fileSeed
	}
	return cfg, seedStr, nil
}
