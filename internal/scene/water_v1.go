package scene

// Water1 is the v1 ocean. It embeds the v0 Water and behaves identically at the
// high vantage (High mode delegates straight through, byte-identical to v0). In low
// (ground-level) mode it stretches the sea further toward the viewer: it reuses the
// same ocean entity (generation is unchanged) and only redraws with a sub-linear
// depth warp and a larger wave scale, taken from the resolved globals
// (Context.Perspective).
//
// FROZEN once released: it keeps the "water" stream key and the WaterV0 schema of
// its embedded v0; add a Water2 for new behavior.
type Water1 struct{ Water }

// Generate is unchanged from v0: the ocean model is the shared global captured into
// the entity regardless of vantage point, so the low look is purely a render-time
// transform.
func (w *Water1) Generate(c *Context) (SceneList, error) {
	return w.Water.Generate(c)
}

// RenderList draws the ocean. In High mode it delegates to the v0 water (byte-
// identical). In Low mode it redraws the same entity with the resolved low-mode
// depth warp and wave scale, so the sea reads as receding to a near eye-level
// horizon.
func (w *Water1) RenderList(c *Context, list SceneList) error {
	if c.Height != Low {
		return w.Water.RenderList(c, list)
	}
	if len(list) == 0 {
		return nil
	}
	oc, err := entityToOcean(list[0])
	if err != nil {
		return err
	}
	return renderOcean(c, oc, c.Perspective.WaterDepthPow, c.Perspective.WaterWaveScale)
}
