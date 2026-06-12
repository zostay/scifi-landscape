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

// ExtractConfig reads the scene file (PNG) at path and returns its embedded
// config as YAML, exactly as stored. It errors if path is not a valid PNG or
// carries no embedded config.
func ExtractConfig(path string) (string, error) {
	texts, err := scenefile.ReadTextsFile(path)
	if err != nil {
		return "", fmt.Errorf("read scene file %q: %w", path, err)
	}
	cfg, ok := texts[scenefile.KeyConfig]
	if !ok {
		return "", fmt.Errorf("scene file %q has no embedded config", path)
	}
	return cfg, nil
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
