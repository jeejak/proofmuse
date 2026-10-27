package meta

// Pay-as-you-go list prices, USD, from https://dev.meta.ai/docs/pricing-rate-limits
// as read on 2026-10-26. Reasoning tokens bill as output tokens.
type price struct {
	InPerM, CachedInPerM, OutPerM float64 // per 1M tokens
	PerImage                      float64 // per image, for per-image models
}

var prices = map[string]price{
	"muse-spark-1.3":             {InPerM: 1.25, CachedInPerM: 0.15, OutPerM: 4.25},
	"muse-spark-1.3-contributor": {InPerM: 0.10, CachedInPerM: 0.10, OutPerM: 0.20},
	"sam-3.1":                    {PerImage: 2.50 / 1000},
}

// Cost prices one call from its reported usage. Unknown models cost 0
// and the caller can see that in the log rather than a made-up number.
func Cost(u Usage) float64 {
	p, ok := prices[u.Model]
	if !ok {
		return 0
	}
	if p.PerImage > 0 {
		return float64(u.Images) * p.PerImage
	}
	uncached := u.InputTokens - u.CachedTokens
	return (float64(uncached)*p.InPerM + float64(u.CachedTokens)*p.CachedInPerM + float64(u.OutputTokens)*p.OutPerM) / 1e6
}
