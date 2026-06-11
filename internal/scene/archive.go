package scene

import (
	"strconv"

	"github.com/zostay/scifi-landscape/internal/config"
	"github.com/zostay/scifi-landscape/internal/scenefile"
)

// SceneTexts assembles the text chunks for a scene file from the reproducibility
// layers. The seed and config are always written; globals and the scene list are
// written only when supplied (pass a nil *Globals or nil list to omit a layer
// that is not yet available). Whatever is written is complete, so a scene file
// reproduces its scene from the highest layer present.
//
// While the element pipeline is only partly migrated to entities, the live app
// records seed+config+globals (all complete for any scene) and omits the scene
// list; the scene list is recorded for the parts that are entity-backed.
func SceneTexts(seed int64, cfg config.Config, g *Globals, list SceneList) (map[string]string, error) {
	texts := map[string]string{
		scenefile.KeySeed: strconv.FormatInt(seed, 10),
	}
	cfgY, err := cfg.Marshal()
	if err != nil {
		return nil, err
	}
	texts[scenefile.KeyConfig] = string(cfgY)

	if g != nil {
		gY, err := g.Marshal()
		if err != nil {
			return nil, err
		}
		texts[scenefile.KeyGlobals] = string(gY)
	}
	if list != nil {
		slY, err := MarshalSceneList(list)
		if err != nil {
			return nil, err
		}
		texts[scenefile.KeySceneList] = string(slY)
	}
	return texts, nil
}

// LoadedScene holds the reproducibility layers parsed from a scene file, with a
// flag for each layer indicating whether the scene file actually carried it.
// Missing layers are filled by the caller (e.g. a missing seed is chosen at
// random, a missing/partial config is completed from defaults).
type LoadedScene struct {
	Seed    int64
	HasSeed bool

	Config    config.Config // always complete (defaults fill any gaps)
	HasConfig bool          // whether the file carried a config at all

	Globals    Globals
	HasGlobals bool

	SceneList    SceneList
	HasSceneList bool
}

// LoadSceneTexts parses the scene-file text chunks into a LoadedScene. The config
// is always completed from defaults (so Config is usable even if the file carried
// only a partial config, or none). Other layers are populated only if present.
func LoadSceneTexts(texts map[string]string) (LoadedScene, error) {
	var ls LoadedScene

	if s, ok := texts[scenefile.KeySeed]; ok {
		n, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			return ls, err
		}
		ls.Seed, ls.HasSeed = n, true
	}

	cfgData, hasConfig := texts[scenefile.KeyConfig]
	cfg, err := config.Load([]byte(cfgData)) // empty data -> defaults
	if err != nil {
		return ls, err
	}
	ls.Config, ls.HasConfig = cfg, hasConfig

	if g, ok := texts[scenefile.KeyGlobals]; ok {
		globals, err := UnmarshalGlobals([]byte(g))
		if err != nil {
			return ls, err
		}
		ls.Globals, ls.HasGlobals = globals, true
	}

	if sl, ok := texts[scenefile.KeySceneList]; ok {
		list, err := UnmarshalSceneList([]byte(sl))
		if err != nil {
			return ls, err
		}
		ls.SceneList, ls.HasSceneList = list, true
	}

	return ls, nil
}
