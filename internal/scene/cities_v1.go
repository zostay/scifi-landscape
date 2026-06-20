package scene

// Cities1 is the v1 city. It embeds the v0 Cities and behaves identically at the
// high vantage (High mode delegates straight through, byte-identical to v0). In low
// (ground-level) mode it keeps the city pinned far-off near the horizon by capping
// its depth band (from the resolved globals, Context.Perspective.CityBandFrac), so
// buildings and their domes do not march down into the stretched foreground.
// Rendering is unchanged — only generation caps the band — so RenderList is
// inherited from the embedded v0.
//
// FROZEN once released: it keeps the "cities" stream key and the CityV0 schema of
// its embedded v0; add a Cities2 for new behavior.
type Cities1 struct{ Cities }

// Generate caps the city's depth band in Low mode (keeping it far-off) and is
// identical to v0 in High mode (band uncapped).
func (c *Cities1) Generate(ctx *Context) (SceneList, error) {
	bandCap := 0.0
	if ctx.Height == Low {
		bandCap = ctx.Perspective.CityBandFrac
	}
	return generateCity(ctx, bandCap)
}
