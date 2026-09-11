// Package api is the HTTP surface: the emulator/app API under /api/v1 (bearer tokens minted
// by the website), and the curation surface under /internal (shared key), until the website
// grows its curation pages (M2).
package api

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/openpak/mods/internal/gate"
	"github.com/openpak/mods/internal/store"
)

type Server struct {
	st          *store.Store
	dataDir     string
	internalKey string
	auth        *websiteAuth
	mux         *http.ServeMux
}

func New(st *store.Store, dataDir, websiteURL, internalKey string) *Server {
	s := &Server{st: st, dataDir: dataDir, internalKey: internalKey, auth: newWebsiteAuth(websiteURL), mux: http.NewServeMux()}
	m := s.mux
	m.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) { w.Write([]byte("ok")) })
	m.HandleFunc("GET /api/v1/titles/{title}/mods", s.handleManifest)
	m.HandleFunc("GET /api/v1/mods/{id}", s.handleMod)
	m.HandleFunc("GET /api/v1/mods/{id}/package", s.handlePackage)
	m.HandleFunc("POST /api/v1/submissions", s.bearer(s.handleSubmit))
	m.HandleFunc("GET /api/v1/me/submissions", s.bearer(s.handleMySubmissions))
	m.HandleFunc("GET /api/v1/me/favourites", s.bearer(s.handleFavourites))
	m.HandleFunc("PUT /api/v1/me/favourites/{id}", s.bearer(s.handleFavourite))
	m.HandleFunc("DELETE /api/v1/me/favourites/{id}", s.bearer(s.handleFavourite))
	m.HandleFunc("GET /internal/queue", s.internal(s.handleQueue))
	m.HandleFunc("POST /internal/mods/{id}/publish", s.internal(s.handleDecision))
	m.HandleFunc("POST /internal/mods/{id}/reject", s.internal(s.handleDecision))
	m.HandleFunc("POST /internal/mods/{id}/takedown", s.internal(s.handleDecision))
	return s
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) { s.mux.ServeHTTP(w, r) }

// ---- auth ----

// websiteAuth resolves a bearer to an account by asking the website's /api/v1/me, which is
// the one place those tokens are minted and revoked. Answers are cached for a minute.
type websiteAuth struct {
	url   string
	mu    sync.Mutex
	cache map[string]cached
}

type cached struct {
	account string
	exp     time.Time
}

func newWebsiteAuth(url string) *websiteAuth {
	return &websiteAuth{url: strings.TrimRight(url, "/"), cache: map[string]cached{}}
}

func (a *websiteAuth) account(ctx context.Context, token string) (string, error) {
	if token == "" {
		return "", errors.New("no token")
	}
	a.mu.Lock()
	if c, ok := a.cache[token]; ok && time.Now().Before(c.exp) {
		a.mu.Unlock()
		return c.account, nil
	}
	a.mu.Unlock()
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, "GET", a.url+"/api/v1/me", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return "", errors.New("token refused")
	}
	var me struct {
		AccountID string `json:"account_id"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 64<<10)).Decode(&me); err != nil || me.AccountID == "" {
		return "", errors.New("no account in reply")
	}
	a.mu.Lock()
	a.cache[token] = cached{account: me.AccountID, exp: time.Now().Add(time.Minute)}
	a.mu.Unlock()
	return me.AccountID, nil
}

type handler func(w http.ResponseWriter, r *http.Request, account string)

func (s *Server) bearer(h handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		account, err := s.auth.account(r.Context(), token)
		if err != nil {
			fail(w, 401, "unauthorized")
			return
		}
		h(w, r, account)
	}
}

func (s *Server) internal(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.internalKey == "" || subtle.ConstantTimeCompare([]byte(r.Header.Get("Authorization")), []byte("Bearer "+s.internalKey)) != 1 {
			fail(w, 401, "unauthorized")
			return
		}
		h(w, r)
	}
}

// ---- catalogue (MOD-03) ----

type entry struct {
	ID        string `json:"id"`
	Slug      string `json:"slug"`
	TitleID   string `json:"title_id"`
	Name      string `json:"name"`
	Version   string `json:"version"`
	Author    string `json:"author"`
	Licence   string `json:"licence"`
	SourceURL string `json:"source_url"`
	Layout    string `json:"layout"`
	Summary   string `json:"summary"`
	SHA256    string `json:"sha256"`
	Size      int64  `json:"size"`
	Status    string `json:"status,omitempty"`
	Reason    string `json:"reason,omitempty"`
	Package   string `json:"package_url"`
}

func view(m *store.Mod, withStatus bool) entry {
	e := entry{ID: m.ID, Slug: m.Slug, TitleID: m.TitleID, Name: m.Name, Version: m.Version, Author: m.Author, Licence: m.Licence,
		SourceURL: m.SourceURL, Layout: m.Layout, Summary: m.Summary, SHA256: m.SHA256, Size: m.Size, Package: "/api/v1/mods/" + m.ID + "/package"}
	if withStatus {
		e.Status, e.Reason = m.Status, m.Reason
	}
	return e
}

func (s *Server) handleManifest(w http.ResponseWriter, r *http.Request) {
	title := strings.ToUpper(r.PathValue("title"))
	mods, err := s.st.Published(r.Context(), title)
	if err != nil {
		fail(w, 500, "internal error")
		return
	}
	out := make([]entry, 0, len(mods))
	for _, m := range mods {
		out = append(out, view(m, false))
	}
	writeJSON(w, map[string]any{"title_id": title, "mods": out})
}

func (s *Server) handleMod(w http.ResponseWriter, r *http.Request) {
	m, err := s.st.Get(r.Context(), r.PathValue("id"))
	if err != nil || !m.Downloadable(time.Now()) {
		fail(w, 404, "no such mod")
		return
	}
	writeJSON(w, view(m, false))
}

func (s *Server) handlePackage(w http.ResponseWriter, r *http.Request) {
	m, err := s.st.Get(r.Context(), r.PathValue("id"))
	if err != nil || !m.Downloadable(time.Now()) {
		fail(w, 404, "no such mod")
		return
	}
	etag := `"` + m.SHA256 + `"`
	w.Header().Set("ETag", etag)
	if r.Header.Get("If-None-Match") == etag {
		w.WriteHeader(http.StatusNotModified)
		return
	}
	f, err := os.Open(s.packagePath(m.ID))
	if err != nil {
		fail(w, 404, "package missing")
		return
	}
	defer f.Close()
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", `attachment; filename="`+m.Slug+"-"+m.Version+`.zip"`)
	http.ServeContent(w, r, m.Slug+".zip", m.CreatedAt, f)
}

func (s *Server) packagePath(id string) string { return filepath.Join(s.dataDir, id+".zip") }

// ---- submission (MOD-01, MOD-02) ----

func (s *Server) handleSubmit(w http.ResponseWriter, r *http.Request, account string) {
	r.Body = http.MaxBytesReader(w, r.Body, gate.MaxPackageBytes+1<<20)
	if err := r.ParseMultipartForm(8 << 20); err != nil {
		fail(w, 400, "multipart form with a \"package\" zip is required")
		return
	}
	f, hdr, err := r.FormFile("package")
	if err != nil {
		fail(w, 400, "multipart field \"package\" is required")
		return
	}
	defer f.Close()
	meta, sum, err := gate.Check(f, hdr.Size)
	if err != nil {
		fail(w, 422, err.Error())
		return
	}
	m := &store.Mod{ID: store.NewID(), Slug: meta.ID, TitleID: meta.TitleID, Name: meta.Name, Version: meta.Version, Author: meta.Author,
		Licence: meta.Licence, SourceURL: meta.Source, Layout: meta.Layout, Summary: meta.Summary, SHA256: sum, Size: hdr.Size, SubmittedBy: account}
	if err := s.storePackage(m.ID, f, hdr.Size); err != nil {
		log.Printf("submit: store package: %v", err)
		fail(w, 500, "could not store the package")
		return
	}
	if err := s.st.Insert(r.Context(), m); err != nil {
		os.Remove(s.packagePath(m.ID))
		log.Printf("submit: insert: %v", err)
		fail(w, 500, "internal error")
		return
	}
	m.Status = "pending"
	w.WriteHeader(http.StatusAccepted)
	writeJSON(w, view(m, true))
}

func (s *Server) storePackage(id string, f io.ReaderAt, size int64) error {
	if err := os.MkdirAll(s.dataDir, 0o755); err != nil {
		return err
	}
	tmp := s.packagePath(id) + ".tmp"
	out, err := os.Create(tmp)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, io.NewSectionReader(f, 0, size)); err != nil {
		out.Close()
		os.Remove(tmp)
		return err
	}
	if err := out.Close(); err != nil {
		os.Remove(tmp)
		return err
	}
	return os.Rename(tmp, s.packagePath(id))
}

func (s *Server) handleMySubmissions(w http.ResponseWriter, r *http.Request, account string) {
	mods, err := s.st.Submitted(r.Context(), account)
	if err != nil {
		fail(w, 500, "internal error")
		return
	}
	out := make([]entry, 0, len(mods))
	for _, m := range mods {
		out = append(out, view(m, true))
	}
	writeJSON(w, map[string]any{"submissions": out})
}

// ---- favourites (MR-1) ----

func (s *Server) handleFavourites(w http.ResponseWriter, r *http.Request, account string) {
	mods, err := s.st.Favourites(r.Context(), account)
	if err != nil {
		fail(w, 500, "internal error")
		return
	}
	out := make([]entry, 0, len(mods))
	for _, m := range mods {
		out = append(out, view(m, false))
	}
	writeJSON(w, map[string]any{"favourites": out})
}

func (s *Server) handleFavourite(w http.ResponseWriter, r *http.Request, account string) {
	id := r.PathValue("id")
	var err error
	if r.Method == "DELETE" {
		err = s.st.RemoveFavourite(r.Context(), account, id)
	} else {
		if m, e := s.st.Get(r.Context(), id); e != nil || !m.Downloadable(time.Now()) {
			fail(w, 404, "no such mod")
			return
		}
		err = s.st.AddFavourite(r.Context(), account, id)
	}
	if err != nil {
		fail(w, 500, "internal error")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ---- curation (MOD-05 until the website has pages) ----

func (s *Server) handleQueue(w http.ResponseWriter, r *http.Request) {
	mods, err := s.st.Queue(r.Context())
	if err != nil {
		fail(w, 500, "internal error")
		return
	}
	out := make([]entry, 0, len(mods))
	for _, m := range mods {
		out = append(out, view(m, true))
	}
	writeJSON(w, map[string]any{"queue": out})
}

func (s *Server) handleDecision(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var body struct {
		Reason string `json:"reason"`
	}
	json.NewDecoder(io.LimitReader(r.Body, 64<<10)).Decode(&body)
	var err error
	switch {
	case strings.HasSuffix(r.URL.Path, "/publish"):
		err = s.st.Publish(r.Context(), id)
	case strings.HasSuffix(r.URL.Path, "/reject"):
		err = s.st.Reject(r.Context(), id, body.Reason)
	default:
		err = s.st.Takedown(r.Context(), id, body.Reason)
	}
	if errors.Is(err, store.ErrNotFound) {
		fail(w, 404, "no such mod")
		return
	}
	if err != nil {
		fail(w, 409, err.Error())
		return
	}
	m, _ := s.st.Get(r.Context(), id)
	writeJSON(w, view(m, true))
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

func fail(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
