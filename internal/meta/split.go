package meta

import "context"

// Split sends structured-output (vision) calls to one OpenAI-compatible
// endpoint and segmentation to another. It lets the eval run Muse Spark
// Open on a partner host such as Baseten while SAM 3.1 stays on the Meta
// Model API, which is the only place SAM is served.
type Split struct {
	Vision    *Client
	Segmenter *Client
}

// JSON goes to the vision endpoint.
func (s Split) JSON(ctx context.Context, r JSONRequest) ([]byte, Usage, error) {
	return s.Vision.JSON(ctx, r)
}

// Segment goes to the segmentation endpoint.
func (s Split) Segment(ctx context.Context, model string, jpeg []byte, phrase string) ([]Detection, Usage, error) {
	return s.Segmenter.Segment(ctx, model, jpeg, phrase)
}

// SetPrice registers a list price for a model this package does not know,
// such as an open-weight model on a partner host, so cost is reported
// instead of zero. Prices are USD per 1M tokens.
func SetPrice(model string, inPerM, outPerM float64) {
	prices[model] = price{InPerM: inPerM, CachedInPerM: inPerM, OutPerM: outPerM}
}
