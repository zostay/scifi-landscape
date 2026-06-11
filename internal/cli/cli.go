// Package cli resolves the scene inputs shared by the windowed app and the
// headless renderer: the configuration and the starting seed, drawn from an
// optional config file and/or an existing scene file.
package cli

import (
	"fmt"
	"os"
	"strconv"

	"github.com/zostay/scifi-landscape/internal/config"
	"github.com/zostay/scifi-landscape/internal/scene"
	"github.com/zostay/scifi-landscape/internal/scenefile"
)

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
