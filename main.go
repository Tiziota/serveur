package main

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"html/template"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

var utf8BOM = []byte{0xEF, 0xBB, 0xBF}

type FileItem struct {
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	URL         string    `json:"url"`
	Description string    `json:"description"`
	AddedAt     time.Time `json:"addedAt"`
}

type Store struct {
	mu    sync.RWMutex
	path  string
	items []FileItem
}

func NewStore(path string) (*Store, error) {
	s := &Store{path: path, items: []FileItem{}}
	if err := s.load(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Store) load() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}

	b, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		s.items = []FileItem{}
		return s.saveLocked()
	}
	if err != nil {
		return err
	}
	b = bytes.TrimPrefix(b, utf8BOM)
	if len(strings.TrimSpace(string(b))) == 0 {
		s.items = []FileItem{}
		return nil
	}

	var items []FileItem
	if err := json.Unmarshal(b, &items); err != nil {
		return err
	}
	s.items = items
	return nil
}

func (s *Store) saveLocked() error {
	b, err := json.MarshalIndent(s.items, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, b, 0o644)
}

func (s *Store) List() []FileItem {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]FileItem, len(s.items))
	copy(out, s.items)
	sort.Slice(out, func(i, j int) bool {
		return out[i].AddedAt.After(out[j].AddedAt)
	})
	return out
}

func (s *Store) Add(item FileItem) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.items = append(s.items, item)
	return s.saveLocked()
}

type App struct {
	store      *Store
	tmpl       *template.Template
	adminUser  string
	adminPass  string
	cookieName string
	sessions   map[string]time.Time
	sMu        sync.RWMutex
}

func newToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

func NewApp(store *Store) (*App, error) {
	t, err := template.ParseFS(os.DirFS("templates"), "index.html")
	if err != nil {
		return nil, err
	}
	return &App{
		store:      store,
		tmpl:       t,
		adminUser:  envOr("ADMIN_USER", "admin"),
		adminPass:  envOr("ADMIN_PASS", "change-me-now"),
		cookieName: "serveur_admin_session",
		sessions:   map[string]time.Time{},
	}, nil
}

func envOr(key, fallback string) string {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return fallback
	}
	return v
}

func (a *App) isAuthed(r *http.Request) bool {
	c, err := r.Cookie(a.cookieName)
	if err != nil {
		return false
	}
	a.sMu.RLock()
	exp, ok := a.sessions[c.Value]
	a.sMu.RUnlock()
	return ok && exp.After(time.Now())
}

func (a *App) createSession(w http.ResponseWriter) error {
	tok, err := newToken()
	if err != nil {
		return err
	}
	exp := time.Now().Add(24 * time.Hour)
	a.sMu.Lock()
	a.sessions[tok] = exp
	a.sMu.Unlock()

	http.SetCookie(w, &http.Cookie{
		Name:     a.cookieName,
		Value:    tok,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   false,
		Expires:  exp,
	})
	return nil
}

func (a *App) clearSession(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(a.cookieName); err == nil {
		a.sMu.Lock()
		delete(a.sessions, c.Value)
		a.sMu.Unlock()
	}
	http.SetCookie(w, &http.Cookie{Name: a.cookieName, Value: "", MaxAge: -1, Path: "/"})
}

func (a *App) home(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	data := map[string]any{
		"IsAuthed": a.isAuthed(r),
	}
	if err := a.tmpl.Execute(w, data); err != nil {
		http.Error(w, "template error", http.StatusInternalServerError)
	}
}

func (a *App) listFiles(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"files": a.store.List()})
}

func (a *App) login(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var payload struct {
		User string `json:"user"`
		Pass string `json:"pass"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}
	if payload.User != a.adminUser || payload.Pass != a.adminPass {
		http.Error(w, "invalid credentials", http.StatusUnauthorized)
		return
	}
	if err := a.createSession(w); err != nil {
		http.Error(w, "session error", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (a *App) logout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	a.clearSession(w, r)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (a *App) addFile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !a.isAuthed(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	var payload struct {
		Title       string `json:"title"`
		URL         string `json:"url"`
		Description string `json:"description"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}
	payload.Title = strings.TrimSpace(payload.Title)
	payload.URL = strings.TrimSpace(payload.URL)
	payload.Description = strings.TrimSpace(payload.Description)
	if payload.Title == "" || payload.URL == "" {
		http.Error(w, "title and url required", http.StatusBadRequest)
		return
	}
	if !strings.HasPrefix(payload.URL, "http://") && !strings.HasPrefix(payload.URL, "https://") {
		http.Error(w, "url must start with http:// or https://", http.StatusBadRequest)
		return
	}
	id, err := newToken()
	if err != nil {
		http.Error(w, "id generation failed", http.StatusInternalServerError)
		return
	}
	item := FileItem{
		ID:          id[:12],
		Title:       payload.Title,
		URL:         payload.URL,
		Description: payload.Description,
		AddedAt:     time.Now().UTC(),
	}
	if err := a.store.Add(item); err != nil {
		http.Error(w, "save failed", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"ok": true, "file": item})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func main() {
	store, err := NewStore(filepath.Join("data", "files.json"))
	if err != nil {
		log.Fatalf("store init error: %v", err)
	}
	app, err := NewApp(store)
	if err != nil {
		log.Fatalf("app init error: %v", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", app.home)
	mux.HandleFunc("/api/files", app.listFiles)
	mux.HandleFunc("/api/admin/login", app.login)
	mux.HandleFunc("/api/admin/logout", app.logout)
	mux.HandleFunc("/api/admin/files", app.addFile)
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))

	port := envOr("PORT", "10000")
	addr := ":" + port
	log.Printf("Serveur running on %s", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatal(err)
	}
}
