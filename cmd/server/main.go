// Command server is the ProofMuse demo: a phone-friendly page where a
// seller picks an order, photographs the packing and gets an advisory
// verdict with Indonesian retake instructions, then retakes and checks
// again.
//
// Environment: MODEL_API_KEY (required for live checks), PORT, EVALSET
// (default "evalset"), DATA_DIR (results cache, default "data"),
// LIMIT_PER_HOUR, LIMIT_PER_DAY, TRUST_PROXY=1 behind a reverse proxy.
package main

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/jeejak/proofmuse/internal/checks"
	"github.com/jeejak/proofmuse/internal/evalset"
	"github.com/jeejak/proofmuse/internal/evidence"
	"github.com/jeejak/proofmuse/internal/meta"
	"github.com/jeejak/proofmuse/internal/pipeline"
)

//go:embed static
var static embed.FS

const (
	maxPhotos     = 8
	maxPhotoBytes = 12 << 20
)

type server struct {
	claim   func(ctx context.Context, text string, photos []evidence.PhotoObs) (*pipeline.ClaimResult, error)
	set     *evalset.Set
	orders  map[string]evalset.Order // by order id
	svc     *checks.Service
	limit   *checks.Limiter
	trust   bool
	samples []sample
	// keys maps an API key to its caller's name (API_KEYS); apiLimit caps
	// each caller's new checks.
	keys     map[string]string
	apiLimit *checks.Limiter
	log      *slog.Logger
}

type sample struct {
	ID      string   `json:"id"`
	Title   string   `json:"title"`
	OrderID string   `json:"order_id"`
	Photos  []string `json:"photos"`
	Retake  []string `json:"retake,omitempty"` // the retaken photo set, same parcel
}

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	set, err := evalset.Load(env("EVALSET", "evalset"))
	if err != nil {
		log.Error("load eval set", "err", err)
		os.Exit(1)
	}
	cfg := pipeline.Defaults()
	key := os.Getenv("MODEL_API_KEY")
	var run checks.Runner = func(context.Context, [][]byte, evidence.Order) (*pipeline.Result, error) {
		return nil, errors.New("MODEL_API_KEY is not set on the server")
	}
	claim := func(context.Context, string, []evidence.PhotoObs) (*pipeline.ClaimResult, error) {
		return nil, errors.New("MODEL_API_KEY is not set on the server")
	}
	if key != "" {
		api := meta.New(key)
		run = func(ctx context.Context, photos [][]byte, o evidence.Order) (*pipeline.Result, error) {
			return pipeline.Run(ctx, api, cfg, photos, o)
		}
		claim = func(ctx context.Context, text string, photos []evidence.PhotoObs) (*pipeline.ClaimResult, error) {
			return pipeline.TriageClaim(ctx, api, cfg, text, photos)
		}
	} else {
		log.Warn("MODEL_API_KEY not set; checks will fail, stored results still serve")
	}
	cfgJSON, _ := json.Marshal(cfg)
	svc, err := checks.New(run, string(cfgJSON), env("DATA_DIR", "data"), 4, log)
	if err != nil {
		log.Error("checks", "err", err)
		os.Exit(1)
	}
	s := &server{
		claim: claim,
		set:   set, orders: map[string]evalset.Order{}, svc: svc, log: log,
		limit:    &checks.Limiter{PerClientHour: envInt("LIMIT_PER_HOUR", 30), PerDay: envInt("LIMIT_PER_DAY", 500)},
		trust:    os.Getenv("TRUST_PROXY") == "1",
		keys:     apiKeys(),
		apiLimit: &checks.Limiter{PerClientHour: envInt("API_LIMIT_PER_HOUR", 600), PerDay: envInt("API_LIMIT_PER_DAY", 2000)},
	}
	if len(s.keys) > 0 {
		log.Info("server-to-server API on", "callers", len(s.keys))
	}
	for _, o := range set.Orders {
		s.orders[o.OrderID] = o
	}
	s.samples = buildSamples(set)

	srv := &http.Server{Addr: ":" + env("PORT", "8080"), Handler: s.routes(), ReadHeaderTimeout: 10 * time.Second}
	go func() {
		log.Info("proofmuse listening", "addr", srv.Addr, "live", key != "")
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("serve", "err", err)
			os.Exit(1)
		}
	}()
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = srv.Shutdown(ctx)
}

func (s *server) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) { io.WriteString(w, "ok\n") })
	// Icons, from the embedded static dir. Browsers ask for /favicon.ico
	// on their own, so it answers with the 32 px PNG.
	for path, file := range map[string]string{
		"/static/jeejak-icon.png": "jeejak-icon.png",
		"/static/favicon-32.png":  "favicon-32.png",
		"/favicon.svg":            "favicon.svg",
		"/favicon.ico":            "favicon-32.png",
		"/apple-touch-icon.png":   "apple-touch-icon.png",
	} {
		mux.HandleFunc("GET "+path, func(w http.ResponseWriter, r *http.Request) {
			b, err := static.ReadFile("static/" + file)
			if err != nil {
				http.NotFound(w, r)
				return
			}
			ct := "image/png"
			if strings.HasSuffix(file, ".svg") {
				ct = "image/svg+xml"
			}
			w.Header().Set("Content-Type", ct)
			w.Header().Set("Cache-Control", "public, max-age=86400")
			w.Write(b)
		})
	}
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		b, _ := static.ReadFile("static/index.html")
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache")
		w.Write(b)
	})
	mux.HandleFunc("GET /api/orders", s.listOrders)
	mux.HandleFunc("GET /api/samples", func(w http.ResponseWriter, _ *http.Request) { writeJSON(w, 200, s.samples) })
	mux.HandleFunc("GET /api/messages", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, 200, map[string]any{"reasons": evidence.Messages, "verdicts": evidence.VerdictText, "advisory": evidence.Advisory,
			"advisory_en": evidence.AdvisoryEN, "claims": evidence.ClaimText, "gaps": evidence.GapText})
	})
	mux.Handle("GET /samples/", http.StripPrefix("/samples/", http.FileServer(http.Dir(filepath.Join(s.set.Dir, "records")))))
	mux.HandleFunc("POST /api/checks", s.createCheck)
	mux.HandleFunc("POST /v1/observe", s.observe)
	mux.HandleFunc("GET /api/checks/{id}", s.getCheck)
	mux.HandleFunc("POST /api/checks/{id}/claim", s.triageClaim)
	mux.HandleFunc("POST /api/checks/{id}/packet", s.packetPDF)
	return securityHeaders(mux)
}

func securityHeaders(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; img-src 'self' blob: data:; style-src 'self' 'unsafe-inline'; script-src 'self' 'unsafe-inline'")
		h.ServeHTTP(w, r)
	})
}

func (s *server) listOrders(w http.ResponseWriter, _ *http.Request) {
	out := make([]evalset.Order, 0, len(s.orders))
	for _, o := range s.orders {
		out = append(out, o)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].OrderID < out[j].OrderID })
	writeJSON(w, 200, out)
}

func (s *server) createCheck(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxPhotos*maxPhotoBytes+1<<20)
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		writeErr(w, 400, "upload too large or malformed")
		return
	}
	defer r.MultipartForm.RemoveAll()
	o, ok := s.orders[r.FormValue("order_id")]
	if !ok {
		writeErr(w, 400, "unknown order_id")
		return
	}
	photos, msg := readPhotos(r)
	if msg != "" {
		writeErr(w, 400, msg)
		return
	}
	order := evidence.Order{Resi: o.Resi, Koli: o.Koli}
	// A result we already have is free: answer it without counting it
	// against the limits.
	id, _ := checks.Key(o.OrderID, s.svc.ConfigKey, photos)
	if c, ok := s.svc.Get(id); !ok || c.Status == checks.Failed {
		if ok, why := s.limit.Allow(s.clientIP(r)); !ok {
			writeErr(w, 429, why)
			return
		}
	}
	c := s.svc.Submit(o.OrderID, order, photos)
	code := http.StatusAccepted
	if c.Status == checks.Done {
		code = http.StatusOK
	}
	writeJSON(w, code, c)
}

func (s *server) getCheck(w http.ResponseWriter, r *http.Request) {
	c, ok := s.svc.Get(r.PathValue("id"))
	if !ok {
		writeErr(w, 404, "no such check")
		return
	}
	writeJSON(w, 200, c)
}

// triageClaim reads a buyer's complaint against a finished check: which
// kind of claim it is, which photos answer it, what is missing.
func (s *server) triageClaim(w http.ResponseWriter, r *http.Request) {
	c, ok := s.svc.Get(r.PathValue("id"))
	if !ok || c.Status != checks.Done || c.Result == nil {
		writeErr(w, 404, "no finished check with that id")
		return
	}
	var body struct {
		Text string `json:"text"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8<<10)).Decode(&body); err != nil || strings.TrimSpace(body.Text) == "" {
		writeErr(w, 400, "send {\"text\": \"...\"}")
		return
	}
	if ok, why := s.limit.Allow(s.clientIP(r)); !ok {
		writeErr(w, 429, why)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), time.Minute)
	defer cancel()
	res, err := s.claim(ctx, strings.TrimSpace(body.Text), c.Result.Photos)
	if err != nil {
		s.log.Warn("claim", "check", c.ID, "err", err)
		writeErr(w, 502, "could not read the claim; try again")
		return
	}
	s.log.Info("claim", "check", c.ID, "type", res.ClaimType, "cost_usd", res.Usage.CostUSD)
	writeJSON(w, 200, res)
}

func (s *server) clientIP(r *http.Request) string {
	if s.trust {
		if f := r.Header.Get("X-Forwarded-For"); f != "" {
			return strings.TrimSpace(strings.Split(f, ",")[0])
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// buildSamples offers the retake pairs (bad set, then the retaken set of
// the same parcel) and a few single cases worth showing.
func buildSamples(set *evalset.Set) []sample {
	photos := func(id string) []string {
		var out []string
		for _, p := range set.Photos[id] {
			out = append(out, "/samples/"+id+"/"+p.File)
		}
		return out
	}
	labels := map[string]evalset.Label{}
	for _, l := range set.Labels {
		labels[l.RecordID] = l
	}
	var out []sample
	pairs := map[string]string{} // bad -> good
	for _, l := range set.Labels {
		if l.RetakeOf != "" {
			pairs[l.RetakeOf] = l.RecordID
		}
	}
	titles := map[string]string{
		"image_dark": "Foto gelap, lalu diulang", "image_blurry": "Foto buram, lalu diulang",
		"label_unreadable": "Resi tidak terbaca, lalu diulang", "not_sealed": "Belum tersegel, lalu diulang",
	}
	var bads []string
	for b := range pairs {
		bads = append(bads, b)
	}
	sort.Strings(bads)
	used := map[string]bool{}
	for _, b := range bads {
		l := labels[b]
		t := "Foto ulang"
		if len(l.Reasons) > 0 {
			if x, ok := titles[l.Reasons[0]]; ok {
				t = x
			}
		}
		if used[t] {
			continue // one sample per kind of problem
		}
		used[t] = true
		out = append(out, sample{ID: b, Title: t, OrderID: set.Orders[b].OrderID, Photos: photos(b), Retake: photos(pairs[b])})
	}
	for _, x := range []struct{ id, title string }{
		{"R39", "Label milik pesanan lain"},
		{"R70", "Label terlalu jauh"},
		{"R04", "Dua koli, bukti lengkap"},
	} {
		if _, ok := labels[x.id]; ok {
			out = append(out, sample{ID: x.id, Title: x.title, OrderID: set.Orders[x.id].OrderID, Photos: photos(x.id)})
		}
	}
	return out
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]string{"error": msg})
}

func env(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func envInt(k string, def int) int {
	if n, err := strconv.Atoi(os.Getenv(k)); err == nil {
		return n
	}
	return def
}
