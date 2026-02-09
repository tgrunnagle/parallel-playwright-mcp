package helpers

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

// FixtureServer serves test HTML fixtures.
type FixtureServer struct {
	t           *testing.T
	server      *httptest.Server
	urls        map[string]string // fixture name -> URL
	fixturesDir string
}

// NewFixtureServer creates and starts a fixture server.
// It serves files from the e2e/fixtures directory.
func NewFixtureServer(t *testing.T) *FixtureServer {
	t.Helper()

	fixturesDir := findFixturesDir(t)

	fs := &FixtureServer{
		t:           t,
		urls:        make(map[string]string),
		fixturesDir: fixturesDir,
	}

	// Create file server handler
	handler := http.FileServer(http.Dir(fixturesDir))

	// Start test server
	fs.server = httptest.NewServer(handler)

	// Build URL map by scanning fixtures directory
	fs.buildURLMap()

	return fs
}

// buildURLMap scans the fixtures directory and builds the URL map.
func (f *FixtureServer) buildURLMap() {
	entries, err := os.ReadDir(f.fixturesDir)
	if err != nil {
		f.t.Logf("Warning: could not read fixtures directory: %v", err)
		return
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			name := entry.Name()
			f.urls[name] = fmt.Sprintf("%s/%s", f.server.URL, name)
		}
	}
}

// URL returns the URL for a named fixture file.
// If the fixture doesn't exist in the map, it constructs a URL anyway.
func (f *FixtureServer) URL(name string) string {
	if url, ok := f.urls[name]; ok {
		return url
	}
	// Construct URL even if not in map
	return fmt.Sprintf("%s/%s", f.server.URL, name)
}

// BaseURL returns the base URL of the fixture server.
func (f *FixtureServer) BaseURL() string {
	return f.server.URL
}

// Close stops the fixture server.
func (f *FixtureServer) Close() {
	if f.server != nil {
		f.server.Close()
	}
}

// findFixturesDir locates the fixtures directory.
func findFixturesDir(t *testing.T) string {
	t.Helper()

	// Get the current working directory
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get working directory: %v", err)
	}

	// Walk up to find the project root
	projectRoot := wd
	for {
		if _, err := os.Stat(filepath.Join(projectRoot, "go.mod")); err == nil {
			break
		}
		parent := filepath.Dir(projectRoot)
		if parent == projectRoot {
			t.Fatalf("Could not find project root (go.mod)")
		}
		projectRoot = parent
	}

	fixturesDir := filepath.Join(projectRoot, "e2e", "fixtures")

	// Create fixtures directory if it doesn't exist
	if err := os.MkdirAll(fixturesDir, 0755); err != nil {
		t.Fatalf("Failed to create fixtures directory: %v", err)
	}

	return fixturesDir
}

// InlineFixtureServer serves dynamically created HTML content.
// Useful for tests that need specific HTML without creating files.
type InlineFixtureServer struct {
	server *httptest.Server
}

// NewInlineFixtureServer creates a server that serves inline HTML content.
func NewInlineFixtureServer(html string) *InlineFixtureServer {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte(html))
	})

	return &InlineFixtureServer{
		server: httptest.NewServer(handler),
	}
}

// URL returns the URL of the inline fixture server.
func (f *InlineFixtureServer) URL() string {
	return f.server.URL
}

// Close stops the inline fixture server.
func (f *InlineFixtureServer) Close() {
	if f.server != nil {
		f.server.Close()
	}
}

// MultiPageFixtureServer serves multiple HTML pages.
type MultiPageFixtureServer struct {
	server *httptest.Server
	pages  map[string]string
}

// NewMultiPageFixtureServer creates a server that serves multiple pages.
// The pages map should be path -> HTML content (e.g., "/page1" -> "<html>...</html>").
func NewMultiPageFixtureServer(pages map[string]string) *MultiPageFixtureServer {
	mux := http.NewServeMux()

	for path, html := range pages {
		content := html // Capture for closure
		mux.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.Write([]byte(content))
		})
	}

	return &MultiPageFixtureServer{
		server: httptest.NewServer(mux),
		pages:  pages,
	}
}

// URL returns the URL for a specific page path.
func (f *MultiPageFixtureServer) URL(path string) string {
	return f.server.URL + path
}

// BaseURL returns the base URL of the server.
func (f *MultiPageFixtureServer) BaseURL() string {
	return f.server.URL
}

// Close stops the multi-page fixture server.
func (f *MultiPageFixtureServer) Close() {
	if f.server != nil {
		f.server.Close()
	}
}
