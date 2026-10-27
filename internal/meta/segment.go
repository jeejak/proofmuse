package meta

import (
	"bufio"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Detection is one SAM 3.1 match: a box in source pixels plus the frame
// size it refers to. The mask is kept as the raw token payload; ProofMuse
// uses boxes for crops, counts and overlays.
type Detection struct {
	ID         int     `json:"id"`
	X1         int     `json:"x1"`
	Y1         int     `json:"y1"`
	X2         int     `json:"x2"`
	Y2         int     `json:"y2"`
	W          int     `json:"w"`
	H          int     `json:"h"`
	Confidence float64 `json:"confidence"`
}

// Grid returns the box on the 0 to 1000 grid Muse Spark and the eval
// labels use, so the two models and the human labels compare directly.
func (d Detection) Grid() [4]int {
	g := func(v, size int) int {
		if size <= 0 {
			return 0
		}
		n := (v*1000 + size/2) / size
		return min(max(n, 0), 1000)
	}
	return [4]int{g(d.X1, d.W), g(d.Y1, d.H), g(d.X2, d.W), g(d.Y2, d.H)}
}

// AreaRatio is the share of the frame the box covers.
func (d Detection) AreaRatio() float64 {
	if d.W <= 0 || d.H <= 0 {
		return 0
	}
	return float64((d.X2-d.X1)*(d.Y2-d.Y1)) / float64(d.W*d.H)
}

// recordRe finds "id<|box;...|>". Box fields are plain key=value pairs
// with no "|" or ">", so matching boxes alone never reads into a mask
// payload, whose encoding alphabet is not documented and may contain
// delimiter characters.
var recordRe = regexp.MustCompile(`(\d+)<\|box;([^|>]*)\|>`)

// ParseSegmentation reads SAM 3.1 output text for a single image.
// Grammar per https://dev.meta.ai/docs/sam/reading-segmentation: the
// frame marker "<0f>", then records "id<|box;k=v;...|><|mask;...|>"
// separated by commas. Box fields are parsed by key, not position.
// Empty text is a valid zero-match answer.
func ParseSegmentation(text string) ([]Detection, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil, nil
	}
	if !strings.HasPrefix(text, "<0f>") {
		return nil, fmt.Errorf("meta: segmentation output does not start with frame 0")
	}
	body := text[len("<0f>"):]
	var out []Detection
	for _, m := range recordRe.FindAllStringSubmatch(body, -1) {
		id, _ := strconv.Atoi(m[1])
		d := Detection{ID: id}
		seen := map[string]bool{}
		for _, kv := range strings.Split(m[2], ";") {
			k, v, ok := strings.Cut(kv, "=")
			if !ok {
				continue
			}
			seen[k] = true
			switch k {
			case "x1", "y1", "x2", "y2", "w", "h":
				n, err := strconv.Atoi(v)
				if err != nil {
					return nil, fmt.Errorf("meta: box %s=%q: %w", k, v, err)
				}
				switch k {
				case "x1":
					d.X1 = n
				case "y1":
					d.Y1 = n
				case "x2":
					d.X2 = n
				case "y2":
					d.Y2 = n
				case "w":
					d.W = n
				case "h":
					d.H = n
				}
			case "c":
				f, err := strconv.ParseFloat(v, 64)
				if err != nil {
					return nil, fmt.Errorf("meta: box c=%q: %w", v, err)
				}
				d.Confidence = f
			}
		}
		for _, k := range []string{"x1", "y1", "x2", "y2", "w", "h"} {
			if !seen[k] {
				return nil, fmt.Errorf("meta: box for object %d has no %s", id, k)
			}
		}
		if d.X2 < d.X1 || d.Y2 < d.Y1 {
			return nil, fmt.Errorf("meta: box for object %d is inverted", id)
		}
		out = append(out, d)
	}
	return out, nil
}

// Segment asks SAM 3.1 for every instance of a noun phrase in one JPEG.
// SAM matching depends on wording: "label" finds a shipping label that
// "shipping label" misses (measured on the eval set, 2026-10-27).
func (c *Client) Segment(ctx context.Context, model string, jpeg []byte, phrase string) ([]Detection, Usage, error) {
	body := map[string]any{
		"model":  model,
		"stream": true,
		"metadata": map[string]string{
			"include_confidence": "true",
			"mask_encoding":      "one_bit",
		},
		"input": []map[string]any{{
			"type": "message",
			"role": "user",
			"content": []map[string]any{
				{"type": "input_text", "text": phrase},
				{"type": "input_image", "image_url": "data:image/jpeg;base64," + base64.StdEncoding.EncodeToString(jpeg)},
			},
		}},
	}
	start := time.Now()
	resp, attempts, err := c.post(ctx, "/responses", body, true)
	usage := Usage{Model: model, Attempts: attempts}
	if err != nil {
		usage.Latency = time.Since(start)
		return nil, usage, err
	}
	defer resp.Body.Close()
	text, images, err := readSSEText(resp.Body)
	usage.Latency = time.Since(start)
	usage.Images = images
	usage.CostUSD = Cost(usage)
	if err != nil {
		return nil, usage, err
	}
	dets, err := ParseSegmentation(text)
	return dets, usage, err
}

// readSSEText collects the final output text of a Responses stream. It
// prefers the server's own "done" text and falls back to the deltas.
func readSSEText(r interface{ Read([]byte) (int, error) }) (text string, images int, err error) {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 1<<20), 64<<20)
	var deltas strings.Builder
	done, completed := "", false
	for sc.Scan() {
		line := sc.Text()
		if !strings.HasPrefix(line, "data: {") {
			continue
		}
		var ev struct {
			Type     string `json:"type"`
			Delta    string `json:"delta"`
			Text     string `json:"text"`
			Response struct {
				Error *struct {
					Message string `json:"message"`
				} `json:"error"`
				Usage struct {
					ImagesProcessed int `json:"images_processed"`
				} `json:"usage"`
			} `json:"response"`
		}
		if err := json.Unmarshal([]byte(line[len("data: "):]), &ev); err != nil {
			return "", 0, fmt.Errorf("meta: bad stream event: %w", err)
		}
		switch ev.Type {
		case "response.output_text.delta":
			deltas.WriteString(ev.Delta)
		case "response.output_text.done":
			done = ev.Text
		case "response.completed":
			completed = true
			images = ev.Response.Usage.ImagesProcessed
		case "response.failed", "error":
			msg := "stream failed"
			if ev.Response.Error != nil {
				msg = ev.Response.Error.Message
			}
			return "", 0, fmt.Errorf("meta: %s", msg)
		}
	}
	if err := sc.Err(); err != nil {
		return "", 0, fmt.Errorf("meta: read stream: %w", err)
	}
	if !completed {
		return "", 0, fmt.Errorf("meta: stream ended before response.completed")
	}
	if done == "" {
		done = deltas.String()
	}
	return done, images, nil
}
