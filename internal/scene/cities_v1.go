package scene

// Cities1 is the v1 city. It embeds the v0 Cities for its stream key and entity
// schema. In low (ground-level) mode it pins the city almost on the horizon by
// capping its depth band (Context.Perspective.CityBandFrac), so it reads as a distant
// skyline — part of the "standing on the ground, seeing less" look. In high mode it
// leaves the band uncapped, exactly like v0, so the elevated view still shows the city
// spread lower down (seeing more). Rendering is unchanged — only generation caps the
// band — so RenderList is inherited from the embedded v0.
//
// FROZEN once released: it keeps the "cities" stream key and the CityV0 schema of
// its embedded v0; add a Cities2 for new behavior.
type Cities1 struct{ Cities }

// Generate caps the city's depth band to the resolved CityBandFrac in low mode (a thin
// strip at the horizon); in high mode the band is uncapped, identical to v0.
func (c *Cities1) Generate(ctx *Context) (SceneList, error) {
	bandCap := 0.0
	if ctx.Height == Low {
		bandCap = ctx.Perspective.CityBandFrac
	}
	return generateCity(ctx, bandCap)
}
