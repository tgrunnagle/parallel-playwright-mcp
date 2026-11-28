//go:build integration

package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mxschmitt/playwright-go"
	"github.com/tgrunnagle/parallel-playwright-mcp/pkg/browser"
	"github.com/tgrunnagle/parallel-playwright-mcp/pkg/session"
)

// startTestHTTPServer starts a local HTTP server that serves simple test pages.
// Each path (/page1, /page2, /page3, etc.) returns a distinct HTML page.
func startTestHTTPServer(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		page := r.URL.Path
		if page == "/" {
			page = "Home"
		}
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprintf(w, "<html><body><h1>%s</h1></body></html>", page)
	})
	mux.HandleFunc("/accessible", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, `<html><body><nav><a href="#">Home</a></nav><main><h1>Title</h1><button aria-label="Submit Form">Submit</button></main></body></html>`)
	})
	mux.HandleFunc("/a11y-semantic", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, `<!DOCTYPE html><html><body>
			<nav aria-label="Main Navigation"><a href="/home">Home</a><a href="/about">About</a></nav>
			<main>
				<h1>Page Title</h1>
				<h2>Section</h2>
				<article><p>Article content</p></article>
				<aside>Sidebar</aside>
			</main>
			<footer>Footer content</footer>
		</body></html>`)
	})
	mux.HandleFunc("/a11y-forms", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, `<!DOCTYPE html><html><body>
			<form>
				<label for="name">Name</label>
				<input id="name" type="text" />
				<label for="email">Email</label>
				<input id="email" type="email" />
				<input type="checkbox" id="agree" aria-label="I agree to terms" />
				<select id="role" aria-label="Select role">
					<option>Admin</option>
					<option>User</option>
				</select>
				<textarea id="bio" aria-label="Biography"></textarea>
				<button type="submit" aria-expanded="false">Submit</button>
			</form>
		</body></html>`)
	})
	mux.HandleFunc("/a11y-hidden", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, `<!DOCTYPE html><html><body>
			<div>
				<p>Visible text</p>
				<p style="display:none">Hidden text</p>
				<p style="visibility:hidden">Invisible text</p>
				<button>Visible Button</button>
				<button style="display:none">Hidden Button</button>
				<button style="visibility:hidden">Invisible Button</button>
				<div aria-hidden="true"><button>Decorative Button</button></div>
			</div>
		</body></html>`)
	})
	mux.HandleFunc("/a11y-password", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, `<!DOCTYPE html><html><body>
			<form>
				<label for="user">Username</label>
				<input id="user" type="text" />
				<label for="pass">Password</label>
				<input id="pass" type="password" />
				<label for="pass-placeholder">PIN</label>
				<input id="pass-placeholder" type="password" placeholder="Enter your PIN" />
				<input id="pass-quoted" type="password" aria-label="Passphrase: 8+ chars" />
				<input id="pass-covered" type="password" aria-label="Covered" style="pointer-events:none" />
				<input id="pass-tiny" type="password" aria-label="Tiny" style="width:0;height:0;padding:0;border:0" />
				<input id="pass-searchbox" type="password" role="searchbox" aria-label="Search key" />
				<input id="pass-combobox" type="password" role="combobox" aria-label="Combo key" />
				<input id="pass-regex" type="password" aria-label="/pw/" />
				<iframe srcdoc="<input id='pass-frame' type='password' aria-label='Frame key' value='frame-secret'>"></iframe>
			</form>
		</body></html>`)
	})
	mux.HandleFunc("/a11y-headings", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, `<!DOCTYPE html><html><body>
			<h1>Level 1</h1>
			<h2>Level 2</h2>
			<h3>Level 3</h3>
			<h4>Level 4</h4>
		</body></html>`)
	})
	mux.HandleFunc("/a11y-lists", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, `<!DOCTYPE html><html><body>
			<ul>
				<li>Apple</li>
				<li>Banana</li>
			</ul>
			<ol>
				<li>First</li>
				<li>Second</li>
			</ol>
		</body></html>`)
	})
	mux.HandleFunc("/a11y-labelledby", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, `<!DOCTYPE html><html><body>
			<h2 id="section-title">Settings</h2>
			<section aria-labelledby="section-title" role="region">
				<p>Configuration options</p>
			</section>
		</body></html>`)
	})
	mux.HandleFunc("/md-headings", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, `<!DOCTYPE html><html><body>
			<h1>Main Title</h1>
			<h2>Subtitle</h2>
			<h3>Section</h3>
			<p>Body text</p>
		</body></html>`)
	})
	mux.HandleFunc("/md-links", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, `<!DOCTYPE html><html><body>
			<p>Visit <a href="https://example.com">Example Site</a> for more info.</p>
		</body></html>`)
	})
	mux.HandleFunc("/md-lists", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, `<!DOCTYPE html><html><body>
			<ul>
				<li>Apple</li>
				<li>Banana</li>
				<li>Cherry</li>
			</ul>
			<ol>
				<li>First</li>
				<li>Second</li>
				<li>Third</li>
			</ol>
		</body></html>`)
	})
	mux.HandleFunc("/md-formatting", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, `<!DOCTYPE html><html><body>
			<p>This is <strong>bold text</strong> and <em>italic text</em>.</p>
		</body></html>`)
	})
	mux.HandleFunc("/md-table", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, `<!DOCTYPE html><html><body>
			<table>
				<thead><tr><th>Name</th><th>Age</th></tr></thead>
				<tbody>
					<tr><td>Alice</td><td>30</td></tr>
					<tr><td>Bob</td><td>25</td></tr>
				</tbody>
			</table>
		</body></html>`)
	})
	mux.HandleFunc("/md-hidden", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, `<!DOCTYPE html><html><body>
			<p>Visible paragraph</p>
			<p style="display:none">Hidden paragraph</p>
			<p style="visibility:hidden">Invisible paragraph</p>
			<div aria-hidden="true"><p>Aria hidden paragraph</p></div>
			<p>Shown <span aria-hidden="true">decorative icon</span>label</p>
			<script>var x = "script content";</script>
			<style>.foo { color: red; }</style>
			<p>Another visible paragraph</p>
		</body></html>`)
	})
	mux.HandleFunc("/md-structure", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, `<!DOCTYPE html><html><body>
			<p><a href="/about">About</a> and <a href="javascript:void(0)">Script link</a></p>
			<p><img src="/a.png" alt="Logo"></p>
			<h2><a href="/x">Docs</a></h2>
			<table id="outer">
				<tr><th>Key</th><th>Detail</th></tr>
				<tr><td>Outer row</td><td><table><tr><td>Inner A</td><td>Inner B</td></tr></table></td></tr>
				<tr><td>Linked</td><td><a href="/cell">Cell link</a></td></tr>
			</table>
		</body></html>`)
	})
	mux.HandleFunc("/md-forms", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, `<!DOCTYPE html><html><body>
			<form>
				<label for="email">Email</label> <input id="email" type="email" value="a@b.c">
				<label><input type="checkbox" checked> Subscribe</label>
				<label><input type="checkbox"> Remember me</label>
				<label for="role">Role</label>
				<select id="role"><option value="a">Admin</option><option value="u" selected>User</option></select>
				<input type="search" aria-label="Search box" value="query">
				<input type="number" placeholder="Quantity" value="3">
				<span id="bio-label">Biography</span> <textarea aria-labelledby="bio-label">Hello there</textarea>
				<input type="radio" name="color" value="red">
				<input type="hidden" name="csrf" value="hidden-token-value">
				<label for="pw">Password</label> <input id="pw" type="password" value="hunter2-secret">
				<label for="pw2">Confirm</label> <input id="pw2" type="password">
				<label for="cur">Current</label> <input id="cur" type="text" autocomplete="current-password" value="shown-secret">
				<label for="otp">Login code</label> <input id="otp" type="text" autocomplete="one-time-code" value="981234">
				<label for="pin">PIN</label> <input id="pin" type="text" style="-webkit-text-security: disc" value="masked-pin">
				<label for="newpw">New</label> <input id="newpw" type="text" autocomplete="new-password" value="new-secret">
				<label for="ccn">Card</label> <input id="ccn" type="text" autocomplete="cc-number" value="4111111111111111">
				<label for="csc">CVC</label> <input id="csc" type="text" autocomplete="cc-csc" value="737">
				<label for="ccn2">Billing card</label> <input id="ccn2" type="text" autocomplete="section-a billing cc-number" value="5555444433331111">
				<label for="otp2">Code</label> <input id="otp2" type="text" autocomplete="One-Time-Code" value="246810">
				<label for="mail">Contact</label> <input id="mail" type="email" autocomplete="email" value="me@example.org">
				<label for="phrase">Recovery phrase</label> <textarea id="phrase" style="-webkit-text-security: disc">secret-phrase</textarea>
				<label for="otparea">Code area</label> <textarea id="otparea" autocomplete="one-time-code">135790</textarea>
				<label for="echo">Echo</label> <input id="echo" aria-labelledby="phrase">
				<pre>pre: <textarea style="-webkit-text-security: disc">pre-secret</textarea></pre>
				<p><code>code: <textarea autocomplete="one-time-code">code-secret</textarea></code></p>
				<span id="pinlbl">Custom PIN</span> <div contenteditable="true" aria-labelledby="pinlbl" style="-webkit-text-security: disc">typed-pin</div>
				<label>Wrapped PIN <div contenteditable="true" style="-webkit-text-security: disc">wrap-secret</div></label>
				<div id="pwed" contenteditable="true" style="-webkit-text-security: disc">labelled-secret</div> <input type="checkbox" aria-labelledby="pwed">
				<div id="pwbox">Box <span contenteditable="true" style="-webkit-text-security: disc">box-secret</span></div> <input type="checkbox" aria-labelledby="pwbox">
				<pre>pre-ed: <span contenteditable="true" style="-webkit-text-security: disc">pre-ed-secret</span></pre>
				<button type="button" id="savebtn">Save draft</button> <input id="draft" aria-labelledby="savebtn">
				<div style="-webkit-text-security: disc"><label><input type="checkbox" checked> Masked box</label> <button type="submit">Masked send</button></div>
				<button type="submit">Send</button>
				<input type="submit" value="Go">
			</form>
		</body></html>`)
	})
	mux.HandleFunc("/md-nested-code", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, "<!DOCTYPE html><html><body>"+
			"<ol><li><pre>  indented\n    more</pre></li></ol>"+
			"<ol><li><p>step</p><pre>  sx</pre></li></ol>"+
			"<ul><li>outer<ul><li><pre>  ny</pre></li></ul></li></ul>"+
			"<blockquote><pre>  quoted</pre></blockquote>"+
			"<blockquote><ul><li><p>s</p><pre>  qx</pre></li></ul></blockquote>"+
			"<p>a   b</p>"+
			"<pre>  x</pre>"+
			"<p>c   d</p>"+
			"<pre>```\ncode\n```\n  after</pre>"+
			"<p>e   f</p>"+
			// Each group puts U+0000 in one element and U+0001 in a later one, with a
			// blank-line run between them. If a source were not cleaned, the pair would
			// become a code segment and the run would survive the tidy pass.
			`<p><span id="a1"></span></p><p>g1</p><p>h1</p><p><span id="b1"></span></p>`+
			`<p><code id="a2"></code></p><p>g2</p><p>h2</p><p><code id="b2"></code></p>`+
			`<p><img id="a3" src="/i.png"></p><p>g3</p><p>h3</p><p><img id="b3" src="/i.png"></p>`+
			`<p><input id="a4" aria-label="First"></p><p>g4</p><p>h4</p><p><input id="b4" aria-label="Second"></p>`+
			`<p><button id="a5">B</button></p><p>g5</p><p>h5</p><p><button id="b5">B</button></p>`+
			`<p><a id="a6">A</a></p><p>g6</p><p>h6</p><p><a id="b6">B</a></p>`+
			`<pre id="p7"></pre>`+
			"<script>"+
			"const nul = String.fromCharCode(0), soh = String.fromCharCode(1);"+
			"const $ = id => document.getElementById(id);"+
			"$('a1').textContent = 'x' + nul; $('b1').textContent = soh + 'x';"+
			"$('a2').textContent = 'c' + nul; $('b2').textContent = soh + 'c';"+
			"$('a3').alt = 'p' + nul; $('b3').alt = soh + 'p';"+
			"$('a4').value = 'v' + nul; $('b4').value = soh + 'v';"+
			"$('a5').setAttribute('aria-label', 'L' + nul); $('b5').setAttribute('aria-label', soh + 'M');"+
			"$('a6').setAttribute('href', 'https://a.example/' + nul); $('b6').setAttribute('href', 'https://b.example/' + soh);"+
			"$('p7').textContent = '  k' + soh + '  m';"+
			"</script>"+
			"</body></html>")
	})
	mux.HandleFunc("/md-shadow", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, `<!DOCTYPE html><html><body>
			<div id="host"><span>Slotted light text</span></div>
			<div id="host2"><p>Unslotted light text</p></div>
			<script>
				document.getElementById('host').attachShadow({mode: 'open'}).innerHTML =
					'<p>Shadow text before</p><slot></slot><p>Shadow text after</p>';
				document.getElementById('host2').attachShadow({mode: 'open'}).innerHTML =
					'<p>Second shadow root</p>';
			</script>
		</body></html>`)
	})
	mux.HandleFunc("/md-iframe", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, `<!DOCTYPE html><html><body>
			<p>Before frames</p>
			<iframe title="Payment form" srcdoc="<p>Inside titled frame</p>"></iframe>
			<iframe src="/md-headings"></iframe>
			<p>After frames</p>
		</body></html>`)
	})
	mux.HandleFunc("/md-code", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, `<!DOCTYPE html><html><body>
			<p>Use <code>fmt.Println</code> to print.</p>
			<pre><code>func main() {
	fmt.Println("hello")
}</code></pre>
		</body></html>`)
	})
	mux.HandleFunc("/md-blockquote", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, `<!DOCTYPE html><html><body>
			<blockquote>This is a quoted passage from a famous author.</blockquote>
		</body></html>`)
	})
	mux.HandleFunc("/md-scoped", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, `<!DOCTYPE html><html><body>
			<header><h1>Page Header</h1></header>
			<main id="content">
				<h2>Main Content</h2>
				<p>Important text</p>
			</main>
			<footer>Footer text</footer>
		</body></html>`)
	})
	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)
	return ts
}

// Integration tests require Playwright runtime to be installed.
// Run with: go test -tags=integration ./pkg/tools/...

// skipIfPlaywrightNotInstalled checks if Playwright is available and skips the test with
// helpful instructions if not.
func skipIfPlaywrightNotInstalled(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		return
	}
	errStr := err.Error()
	if strings.Contains(errStr, "please install the driver") ||
		strings.Contains(errStr, "executable file not found") ||
		strings.Contains(errStr, "Playwright") ||
		strings.Contains(errStr, "browser pool is not running") {
		t.Skipf("Playwright not installed or pool not started. Run 'task playwright:install' to install browsers.\nOriginal error: %v", err)
	}
}

func setupPoolAndManager(t *testing.T) (browser.BrowserPool, session.BrowserSessionManager) {
	t.Helper()
	pool := browser.NewBrowserPoolWithOptions(browser.PoolOptions{
		DefaultHeadless: true,
	})
	ctx := context.Background()
	if err := pool.Start(ctx); err != nil {
		skipIfPlaywrightNotInstalled(t, err)
		t.Fatalf("Failed to start pool: %v", err)
	}
	mgr := session.NewManager(pool)
	return pool, mgr
}

func TestSessionCreateToolIntegration(t *testing.T) {
	pool, mgr := setupPoolAndManager(t)
	defer pool.Stop(context.Background())

	handler := SessionCreateHandler(mgr, DefaultTimeoutConfig())
	ctx := context.Background()

	t.Run("creates session with default options", func(t *testing.T) {
		req := mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name:      "session_create",
				Arguments: map[string]any{},
			},
		}

		result, err := handler(ctx, req)
		if err != nil {
			t.Fatalf("Handler returned error: %v", err)
		}

		if result.IsError {
			t.Fatalf("Expected success, got error: %s", extractTextContent(result.Content))
		}

		text := extractTextContent(result.Content)
		if !strings.HasPrefix(text, "Created session: sess-") {
			t.Errorf("Expected session ID in response, got: %s", text)
		}

		// Extract session ID and verify it exists
		sessionID := strings.TrimPrefix(text, "Created session: ")
		sessions := mgr.ListSessions("")
		found := false
		for _, s := range sessions {
			if s.ID == sessionID {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Session %s not found in manager", sessionID)
		}

		// Clean up
		mgr.CloseSession(ctx, "", sessionID)
	})

	t.Run("creates session with chromium", func(t *testing.T) {
		req := mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name: "session_create",
				Arguments: map[string]any{
					"browserType": "chromium",
				},
			},
		}

		result, err := handler(ctx, req)
		if err != nil {
			t.Fatalf("Handler returned error: %v", err)
		}

		if result.IsError {
			skipIfPlaywrightNotInstalled(t, extractErrorFromResult(result))
			t.Fatalf("Expected success, got error: %s", extractTextContent(result.Content))
		}

		text := extractTextContent(result.Content)
		sessionID := strings.TrimPrefix(text, "Created session: ")

		// Verify browser type
		sessions := mgr.ListSessions("")
		for _, s := range sessions {
			if s.ID == sessionID {
				if s.BrowserType != browser.BrowserChromium {
					t.Errorf("Expected chromium, got %s", s.BrowserType)
				}
				break
			}
		}

		// Clean up
		mgr.CloseSession(ctx, "", sessionID)
	})

	t.Run("creates session with custom viewport", func(t *testing.T) {
		req := mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name: "session_create",
				Arguments: map[string]any{
					"viewport": map[string]any{
						"width":  float64(1920),
						"height": float64(1080),
					},
				},
			},
		}

		result, err := handler(ctx, req)
		if err != nil {
			t.Fatalf("Handler returned error: %v", err)
		}

		if result.IsError {
			skipIfPlaywrightNotInstalled(t, extractErrorFromResult(result))
			t.Fatalf("Expected success, got error: %s", extractTextContent(result.Content))
		}

		text := extractTextContent(result.Content)
		sessionID := strings.TrimPrefix(text, "Created session: ")

		// Clean up
		mgr.CloseSession(ctx, "", sessionID)
	})
}

func TestSessionListToolIntegration(t *testing.T) {
	pool, mgr := setupPoolAndManager(t)
	defer pool.Stop(context.Background())

	createHandler := SessionCreateHandler(mgr, DefaultTimeoutConfig())
	listHandler := SessionListHandler(mgr, DefaultTimeoutConfig())
	ctx := context.Background()

	t.Run("lists empty sessions for new connection", func(t *testing.T) {
		req := mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name:      "session_list",
				Arguments: map[string]any{},
			},
		}

		result, err := listHandler(ctx, req)
		if err != nil {
			t.Fatalf("Handler returned error: %v", err)
		}

		if result.IsError {
			t.Fatalf("Expected success, got error: %s", extractTextContent(result.Content))
		}

		text := extractTextContent(result.Content)
		var response SessionListResponse
		if err := json.Unmarshal([]byte(text), &response); err != nil {
			t.Fatalf("Failed to parse response: %v", err)
		}

		if len(response.Sessions) != 0 {
			t.Errorf("Expected empty sessions, got %d", len(response.Sessions))
		}
	})

	t.Run("lists created sessions", func(t *testing.T) {
		// Create a session
		createReq := mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name:      "session_create",
				Arguments: map[string]any{},
			},
		}

		createResult, err := createHandler(ctx, createReq)
		if err != nil {
			t.Fatalf("Create handler returned error: %v", err)
		}
		if createResult.IsError {
			skipIfPlaywrightNotInstalled(t, extractErrorFromResult(createResult))
			t.Fatalf("Create failed: %s", extractTextContent(createResult.Content))
		}

		sessionID := strings.TrimPrefix(extractTextContent(createResult.Content), "Created session: ")
		defer mgr.CloseSession(ctx, "", sessionID)

		// List sessions
		listReq := mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name:      "session_list",
				Arguments: map[string]any{},
			},
		}

		listResult, err := listHandler(ctx, listReq)
		if err != nil {
			t.Fatalf("List handler returned error: %v", err)
		}

		if listResult.IsError {
			t.Fatalf("Expected success, got error: %s", extractTextContent(listResult.Content))
		}

		text := extractTextContent(listResult.Content)
		var response SessionListResponse
		if err := json.Unmarshal([]byte(text), &response); err != nil {
			t.Fatalf("Failed to parse response: %v", err)
		}

		if len(response.Sessions) != 1 {
			t.Errorf("Expected 1 session, got %d", len(response.Sessions))
		}

		if response.Sessions[0].SessionID != sessionID {
			t.Errorf("Expected session ID %s, got %s", sessionID, response.Sessions[0].SessionID)
		}
	})
}

func TestSessionCloseToolIntegration(t *testing.T) {
	pool, mgr := setupPoolAndManager(t)
	defer pool.Stop(context.Background())

	createHandler := SessionCreateHandler(mgr, DefaultTimeoutConfig())
	closeHandler := SessionCloseHandler(mgr, DefaultTimeoutConfig())
	listHandler := SessionListHandler(mgr, DefaultTimeoutConfig())
	ctx := context.Background()

	t.Run("closes session and removes from list", func(t *testing.T) {
		// Create a session
		createReq := mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name:      "session_create",
				Arguments: map[string]any{},
			},
		}

		createResult, err := createHandler(ctx, createReq)
		if err != nil {
			t.Fatalf("Create handler returned error: %v", err)
		}
		if createResult.IsError {
			skipIfPlaywrightNotInstalled(t, extractErrorFromResult(createResult))
			t.Fatalf("Create failed: %s", extractTextContent(createResult.Content))
		}

		sessionID := strings.TrimPrefix(extractTextContent(createResult.Content), "Created session: ")

		// Close the session
		closeReq := mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name: "session_close",
				Arguments: map[string]any{
					"sessionId": sessionID,
				},
			},
		}

		closeResult, err := closeHandler(ctx, closeReq)
		if err != nil {
			t.Fatalf("Close handler returned error: %v", err)
		}

		if closeResult.IsError {
			t.Fatalf("Expected success, got error: %s", extractTextContent(closeResult.Content))
		}

		text := extractTextContent(closeResult.Content)
		if !strings.Contains(text, sessionID) {
			t.Errorf("Expected session ID in close response, got: %s", text)
		}

		// Verify session is removed from list
		listReq := mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name:      "session_list",
				Arguments: map[string]any{},
			},
		}

		listResult, err := listHandler(ctx, listReq)
		if err != nil {
			t.Fatalf("List handler returned error: %v", err)
		}

		listText := extractTextContent(listResult.Content)
		var response SessionListResponse
		if err := json.Unmarshal([]byte(listText), &response); err != nil {
			t.Fatalf("Failed to parse response: %v", err)
		}

		for _, s := range response.Sessions {
			if s.SessionID == sessionID {
				t.Errorf("Session %s should have been removed", sessionID)
			}
		}
	})

	t.Run("returns error for non-existent session", func(t *testing.T) {
		closeReq := mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name: "session_close",
				Arguments: map[string]any{
					"sessionId": "sess-non-existent",
				},
			},
		}

		closeResult, err := closeHandler(ctx, closeReq)
		if err != nil {
			t.Fatalf("Close handler returned error: %v", err)
		}

		if !closeResult.IsError {
			t.Error("Expected error for non-existent session")
		}

		text := extractTextContent(closeResult.Content)
		if !strings.Contains(text, "not found") {
			t.Errorf("Expected 'not found' in error message, got: %s", text)
		}
	})
}

func TestFullSessionLifecycleIntegration(t *testing.T) {
	pool, mgr := setupPoolAndManager(t)
	defer pool.Stop(context.Background())

	createHandler := SessionCreateHandler(mgr, DefaultTimeoutConfig())
	listHandler := SessionListHandler(mgr, DefaultTimeoutConfig())
	closeHandler := SessionCloseHandler(mgr, DefaultTimeoutConfig())
	ctx := context.Background()

	t.Run("full create -> list -> close flow", func(t *testing.T) {
		// Step 1: List sessions (should be empty)
		listReq := mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name:      "session_list",
				Arguments: map[string]any{},
			},
		}
		listResult, _ := listHandler(ctx, listReq)
		text := extractTextContent(listResult.Content)
		var initialList SessionListResponse
		json.Unmarshal([]byte(text), &initialList)
		initialCount := len(initialList.Sessions)

		// Step 2: Create first session
		createReq1 := mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name: "session_create",
				Arguments: map[string]any{
					"browserType": "chromium",
				},
			},
		}
		createResult1, err := createHandler(ctx, createReq1)
		if err != nil || createResult1.IsError {
			skipIfPlaywrightNotInstalled(t, extractErrorFromResult(createResult1))
			t.Fatalf("Create first session failed: %v", extractTextContent(createResult1.Content))
		}
		sessionID1 := strings.TrimPrefix(extractTextContent(createResult1.Content), "Created session: ")

		// Step 3: Create second session
		createReq2 := mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name:      "session_create",
				Arguments: map[string]any{},
			},
		}
		createResult2, err := createHandler(ctx, createReq2)
		if err != nil || createResult2.IsError {
			t.Fatalf("Create second session failed: %v", extractTextContent(createResult2.Content))
		}
		sessionID2 := strings.TrimPrefix(extractTextContent(createResult2.Content), "Created session: ")

		// Step 4: List sessions (should have 2 new sessions)
		listResult2, _ := listHandler(ctx, listReq)
		text = extractTextContent(listResult2.Content)
		var afterCreate SessionListResponse
		json.Unmarshal([]byte(text), &afterCreate)
		if len(afterCreate.Sessions) != initialCount+2 {
			t.Errorf("Expected %d sessions after create, got %d", initialCount+2, len(afterCreate.Sessions))
		}

		// Step 5: Close first session
		closeReq1 := mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name: "session_close",
				Arguments: map[string]any{
					"sessionId": sessionID1,
				},
			},
		}
		closeResult1, _ := closeHandler(ctx, closeReq1)
		if closeResult1.IsError {
			t.Errorf("Close first session failed: %s", extractTextContent(closeResult1.Content))
		}

		// Step 6: List sessions (should have 1 less)
		listResult3, _ := listHandler(ctx, listReq)
		text = extractTextContent(listResult3.Content)
		var afterClose1 SessionListResponse
		json.Unmarshal([]byte(text), &afterClose1)
		if len(afterClose1.Sessions) != initialCount+1 {
			t.Errorf("Expected %d sessions after first close, got %d", initialCount+1, len(afterClose1.Sessions))
		}

		// Verify first session is gone
		for _, s := range afterClose1.Sessions {
			if s.SessionID == sessionID1 {
				t.Error("First session should have been removed")
			}
		}

		// Verify second session still exists
		found := false
		for _, s := range afterClose1.Sessions {
			if s.SessionID == sessionID2 {
				found = true
				break
			}
		}
		if !found {
			t.Error("Second session should still exist")
		}

		// Step 7: Close second session
		closeReq2 := mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name: "session_close",
				Arguments: map[string]any{
					"sessionId": sessionID2,
				},
			},
		}
		closeResult2, _ := closeHandler(ctx, closeReq2)
		if closeResult2.IsError {
			t.Errorf("Close second session failed: %s", extractTextContent(closeResult2.Content))
		}

		// Step 8: List sessions (should be back to initial)
		listResult4, _ := listHandler(ctx, listReq)
		text = extractTextContent(listResult4.Content)
		var finalList SessionListResponse
		json.Unmarshal([]byte(text), &finalList)
		if len(finalList.Sessions) != initialCount {
			t.Errorf("Expected %d sessions at end, got %d", initialCount, len(finalList.Sessions))
		}
	})
}

// extractErrorFromResult extracts an error from a tool result for skip checking.
func extractErrorFromResult(result *mcp.CallToolResult) error {
	if result == nil || !result.IsError {
		return nil
	}
	text := extractTextContent(result.Content)
	return &resultError{message: text}
}

type resultError struct {
	message string
}

func (e *resultError) Error() string {
	return e.message
}

// TestNavigateToolIntegration tests the navigate tool with real browser instances.
func TestNavigateToolIntegration(t *testing.T) {
	pool, mgr := setupPoolAndManager(t)
	defer pool.Stop(context.Background())

	createHandler := SessionCreateHandler(mgr, DefaultTimeoutConfig())
	navigateHandler := NavigateHandler(mgr, DefaultTimeoutConfig(), nil)
	ctx := context.Background()

	// Create a session for all tests
	createReq := mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name:      "session_create",
			Arguments: map[string]any{},
		},
	}
	createResult, err := createHandler(ctx, createReq)
	if err != nil {
		t.Fatalf("Create handler returned error: %v", err)
	}
	if createResult.IsError {
		skipIfPlaywrightNotInstalled(t, extractErrorFromResult(createResult))
		t.Fatalf("Create failed: %s", extractTextContent(createResult.Content))
	}
	sessionID := strings.TrimPrefix(extractTextContent(createResult.Content), "Created session: ")
	defer mgr.CloseSession(ctx, "", sessionID)

	t.Run("navigates to valid URL", func(t *testing.T) {
		req := mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name: "navigate",
				Arguments: map[string]any{
					"sessionId": sessionID,
					"url":       "about:blank",
				},
			},
		}

		result, err := navigateHandler(ctx, req)
		if err != nil {
			t.Fatalf("Handler returned error: %v", err)
		}

		if result.IsError {
			t.Fatalf("Expected success, got error: %s", extractTextContent(result.Content))
		}

		text := extractTextContent(result.Content)
		if !strings.Contains(text, "Navigated to") {
			t.Errorf("Expected navigation success message, got: %s", text)
		}
	})

	t.Run("navigates with waitUntil option", func(t *testing.T) {
		req := mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name: "navigate",
				Arguments: map[string]any{
					"sessionId": sessionID,
					"url":       "about:blank",
					"waitUntil": "load",
				},
			},
		}

		result, err := navigateHandler(ctx, req)
		if err != nil {
			t.Fatalf("Handler returned error: %v", err)
		}

		if result.IsError {
			t.Fatalf("Expected success, got error: %s", extractTextContent(result.Content))
		}
	})

	t.Run("returns error for invalid session", func(t *testing.T) {
		req := mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name: "navigate",
				Arguments: map[string]any{
					"sessionId": "sess-invalid",
					"url":       "about:blank",
				},
			},
		}

		result, err := navigateHandler(ctx, req)
		if err != nil {
			t.Fatalf("Handler returned error: %v", err)
		}

		if !result.IsError {
			t.Error("Expected error for invalid session")
		}

		text := extractTextContent(result.Content)
		if !strings.Contains(text, "[-32001]") {
			t.Errorf("Expected session not found error code, got: %s", text)
		}
	})
}

// TestGoBackToolIntegration tests the go_back tool with real browser instances.
func TestGoBackToolIntegration(t *testing.T) {
	pool, mgr := setupPoolAndManager(t)
	defer pool.Stop(context.Background())

	createHandler := SessionCreateHandler(mgr, DefaultTimeoutConfig())
	navigateHandler := NavigateHandler(mgr, DefaultTimeoutConfig(), nil)
	goBackHandler := GoBackHandler(mgr, DefaultTimeoutConfig(), nil)
	ctx := context.Background()

	// Create a session
	createReq := mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name:      "session_create",
			Arguments: map[string]any{},
		},
	}
	createResult, err := createHandler(ctx, createReq)
	if err != nil {
		t.Fatalf("Create handler returned error: %v", err)
	}
	if createResult.IsError {
		skipIfPlaywrightNotInstalled(t, extractErrorFromResult(createResult))
		t.Fatalf("Create failed: %s", extractTextContent(createResult.Content))
	}
	sessionID := strings.TrimPrefix(extractTextContent(createResult.Content), "Created session: ")
	defer mgr.CloseSession(ctx, "", sessionID)

	t.Run("returns no history message on fresh session", func(t *testing.T) {
		req := mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name: "go_back",
				Arguments: map[string]any{
					"sessionId": sessionID,
				},
			},
		}

		result, err := goBackHandler(ctx, req)
		if err != nil {
			t.Fatalf("Handler returned error: %v", err)
		}

		if result.IsError {
			t.Fatalf("Expected success, got error: %s", extractTextContent(result.Content))
		}

		text := extractTextContent(result.Content)
		if !strings.Contains(text, "No previous page") {
			t.Errorf("Expected 'No previous page' message, got: %s", text)
		}
	})

	t.Run("navigates back after page navigation", func(t *testing.T) {
		ts := startTestHTTPServer(t)

		// Navigate to first page
		navReq := mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name: "navigate",
				Arguments: map[string]any{
					"sessionId": sessionID,
					"url":       ts.URL + "/page1",
				},
			},
		}
		navigateHandler(ctx, navReq)

		// Navigate to second page
		navReq2 := mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name: "navigate",
				Arguments: map[string]any{
					"sessionId": sessionID,
					"url":       ts.URL + "/page2",
				},
			},
		}
		navigateHandler(ctx, navReq2)

		// Go back
		backReq := mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name: "go_back",
				Arguments: map[string]any{
					"sessionId": sessionID,
				},
			},
		}

		result, err := goBackHandler(ctx, backReq)
		if err != nil {
			t.Fatalf("Handler returned error: %v", err)
		}

		if result.IsError {
			t.Fatalf("Expected success, got error: %s", extractTextContent(result.Content))
		}

		text := extractTextContent(result.Content)
		if !strings.Contains(text, "Navigated back") {
			t.Errorf("Expected 'Navigated back' message, got: %s", text)
		}
	})
}

// TestGoForwardToolIntegration tests the go_forward tool with real browser instances.
func TestGoForwardToolIntegration(t *testing.T) {
	pool, mgr := setupPoolAndManager(t)
	defer pool.Stop(context.Background())

	createHandler := SessionCreateHandler(mgr, DefaultTimeoutConfig())
	navigateHandler := NavigateHandler(mgr, DefaultTimeoutConfig(), nil)
	goBackHandler := GoBackHandler(mgr, DefaultTimeoutConfig(), nil)
	goForwardHandler := GoForwardHandler(mgr, DefaultTimeoutConfig(), nil)
	ctx := context.Background()

	// Create a session
	createReq := mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name:      "session_create",
			Arguments: map[string]any{},
		},
	}
	createResult, err := createHandler(ctx, createReq)
	if err != nil {
		t.Fatalf("Create handler returned error: %v", err)
	}
	if createResult.IsError {
		skipIfPlaywrightNotInstalled(t, extractErrorFromResult(createResult))
		t.Fatalf("Create failed: %s", extractTextContent(createResult.Content))
	}
	sessionID := strings.TrimPrefix(extractTextContent(createResult.Content), "Created session: ")
	defer mgr.CloseSession(ctx, "", sessionID)

	t.Run("returns no history message on fresh session", func(t *testing.T) {
		req := mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name: "go_forward",
				Arguments: map[string]any{
					"sessionId": sessionID,
				},
			},
		}

		result, err := goForwardHandler(ctx, req)
		if err != nil {
			t.Fatalf("Handler returned error: %v", err)
		}

		if result.IsError {
			t.Fatalf("Expected success, got error: %s", extractTextContent(result.Content))
		}

		text := extractTextContent(result.Content)
		if !strings.Contains(text, "No forward page") {
			t.Errorf("Expected 'No forward page' message, got: %s", text)
		}
	})

	t.Run("navigates forward after go_back", func(t *testing.T) {
		ts := startTestHTTPServer(t)

		// Navigate to first page
		navReq := mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name: "navigate",
				Arguments: map[string]any{
					"sessionId": sessionID,
					"url":       ts.URL + "/page1",
				},
			},
		}
		navigateHandler(ctx, navReq)

		// Navigate to second page
		navReq2 := mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name: "navigate",
				Arguments: map[string]any{
					"sessionId": sessionID,
					"url":       ts.URL + "/page2",
				},
			},
		}
		navigateHandler(ctx, navReq2)

		// Go back
		backReq := mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name: "go_back",
				Arguments: map[string]any{
					"sessionId": sessionID,
				},
			},
		}
		goBackHandler(ctx, backReq)

		// Go forward
		forwardReq := mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name: "go_forward",
				Arguments: map[string]any{
					"sessionId": sessionID,
				},
			},
		}

		result, err := goForwardHandler(ctx, forwardReq)
		if err != nil {
			t.Fatalf("Handler returned error: %v", err)
		}

		if result.IsError {
			t.Fatalf("Expected success, got error: %s", extractTextContent(result.Content))
		}

		text := extractTextContent(result.Content)
		if !strings.Contains(text, "Navigated forward") {
			t.Errorf("Expected 'Navigated forward' message, got: %s", text)
		}
	})
}

// TestReloadToolIntegration tests the reload tool with real browser instances.
func TestReloadToolIntegration(t *testing.T) {
	pool, mgr := setupPoolAndManager(t)
	defer pool.Stop(context.Background())

	createHandler := SessionCreateHandler(mgr, DefaultTimeoutConfig())
	navigateHandler := NavigateHandler(mgr, DefaultTimeoutConfig(), nil)
	reloadHandler := ReloadHandler(mgr, DefaultTimeoutConfig(), nil)
	ctx := context.Background()

	// Create a session
	createReq := mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name:      "session_create",
			Arguments: map[string]any{},
		},
	}
	createResult, err := createHandler(ctx, createReq)
	if err != nil {
		t.Fatalf("Create handler returned error: %v", err)
	}
	if createResult.IsError {
		skipIfPlaywrightNotInstalled(t, extractErrorFromResult(createResult))
		t.Fatalf("Create failed: %s", extractTextContent(createResult.Content))
	}
	sessionID := strings.TrimPrefix(extractTextContent(createResult.Content), "Created session: ")
	defer mgr.CloseSession(ctx, "", sessionID)

	// Navigate to a page first
	navReq := mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name: "navigate",
			Arguments: map[string]any{
				"sessionId": sessionID,
				"url":       "about:blank",
			},
		},
	}
	navigateHandler(ctx, navReq)

	t.Run("reloads current page", func(t *testing.T) {
		req := mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name: "reload",
				Arguments: map[string]any{
					"sessionId": sessionID,
				},
			},
		}

		result, err := reloadHandler(ctx, req)
		if err != nil {
			t.Fatalf("Handler returned error: %v", err)
		}

		if result.IsError {
			t.Fatalf("Expected success, got error: %s", extractTextContent(result.Content))
		}

		text := extractTextContent(result.Content)
		if !strings.Contains(text, "Reloaded page") {
			t.Errorf("Expected 'Reloaded page' message, got: %s", text)
		}
	})

	t.Run("reloads with waitUntil option", func(t *testing.T) {
		req := mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name: "reload",
				Arguments: map[string]any{
					"sessionId": sessionID,
					"waitUntil": "domcontentloaded",
				},
			},
		}

		result, err := reloadHandler(ctx, req)
		if err != nil {
			t.Fatalf("Handler returned error: %v", err)
		}

		if result.IsError {
			t.Fatalf("Expected success, got error: %s", extractTextContent(result.Content))
		}
	})
}

// TestNavigationFlowIntegration tests a complete navigation workflow.
func TestNavigationFlowIntegration(t *testing.T) {
	pool, mgr := setupPoolAndManager(t)
	defer pool.Stop(context.Background())

	createHandler := SessionCreateHandler(mgr, DefaultTimeoutConfig())
	navigateHandler := NavigateHandler(mgr, DefaultTimeoutConfig(), nil)
	goBackHandler := GoBackHandler(mgr, DefaultTimeoutConfig(), nil)
	goForwardHandler := GoForwardHandler(mgr, DefaultTimeoutConfig(), nil)
	reloadHandler := ReloadHandler(mgr, DefaultTimeoutConfig(), nil)
	ctx := context.Background()

	// Create a session
	createReq := mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name:      "session_create",
			Arguments: map[string]any{},
		},
	}
	createResult, err := createHandler(ctx, createReq)
	if err != nil {
		t.Fatalf("Create handler returned error: %v", err)
	}
	if createResult.IsError {
		skipIfPlaywrightNotInstalled(t, extractErrorFromResult(createResult))
		t.Fatalf("Create failed: %s", extractTextContent(createResult.Content))
	}
	sessionID := strings.TrimPrefix(extractTextContent(createResult.Content), "Created session: ")
	defer mgr.CloseSession(ctx, "", sessionID)

	t.Run("full navigation flow: navigate -> navigate -> back -> forward -> reload", func(t *testing.T) {
		ts := startTestHTTPServer(t)

		// Step 1: Navigate to first page
		navReq1 := mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name: "navigate",
				Arguments: map[string]any{
					"sessionId": sessionID,
					"url":       ts.URL + "/page1",
				},
			},
		}
		result, _ := navigateHandler(ctx, navReq1)
		if result.IsError {
			t.Fatalf("Navigate to page 1 failed: %s", extractTextContent(result.Content))
		}

		// Step 2: Navigate to second page
		navReq2 := mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name: "navigate",
				Arguments: map[string]any{
					"sessionId": sessionID,
					"url":       ts.URL + "/page2",
				},
			},
		}
		result, _ = navigateHandler(ctx, navReq2)
		if result.IsError {
			t.Fatalf("Navigate to page 2 failed: %s", extractTextContent(result.Content))
		}

		// Step 3: Navigate to third page
		navReq3 := mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name: "navigate",
				Arguments: map[string]any{
					"sessionId": sessionID,
					"url":       ts.URL + "/page3",
				},
			},
		}
		result, _ = navigateHandler(ctx, navReq3)
		if result.IsError {
			t.Fatalf("Navigate to page 3 failed: %s", extractTextContent(result.Content))
		}

		// Step 4: Go back (should be at page 2)
		backReq := mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name: "go_back",
				Arguments: map[string]any{
					"sessionId": sessionID,
				},
			},
		}
		result, _ = goBackHandler(ctx, backReq)
		if result.IsError {
			t.Fatalf("Go back failed: %s", extractTextContent(result.Content))
		}
		text := extractTextContent(result.Content)
		if !strings.Contains(text, "Navigated back") {
			t.Errorf("Expected back navigation message, got: %s", text)
		}

		// Step 5: Go forward (should be at page 3)
		forwardReq := mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name: "go_forward",
				Arguments: map[string]any{
					"sessionId": sessionID,
				},
			},
		}
		result, _ = goForwardHandler(ctx, forwardReq)
		if result.IsError {
			t.Fatalf("Go forward failed: %s", extractTextContent(result.Content))
		}
		text = extractTextContent(result.Content)
		if !strings.Contains(text, "Navigated forward") {
			t.Errorf("Expected forward navigation message, got: %s", text)
		}

		// Step 6: Reload current page
		reloadReq := mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name: "reload",
				Arguments: map[string]any{
					"sessionId": sessionID,
				},
			},
		}
		result, _ = reloadHandler(ctx, reloadReq)
		if result.IsError {
			t.Fatalf("Reload failed: %s", extractTextContent(result.Content))
		}
		text = extractTextContent(result.Content)
		if !strings.Contains(text, "Reloaded page") {
			t.Errorf("Expected reload message, got: %s", text)
		}
	})
}

// TestScreenshotToolIntegration tests the screenshot tool with real browser instances.
func TestScreenshotToolIntegration(t *testing.T) {
	pool, mgr := setupPoolAndManager(t)
	defer pool.Stop(context.Background())

	createHandler := SessionCreateHandler(mgr, DefaultTimeoutConfig())
	navigateHandler := NavigateHandler(mgr, DefaultTimeoutConfig(), nil)
	screenshotHandler := ScreenshotHandler(mgr, DefaultTimeoutConfig())
	ctx := context.Background()

	// Create a session
	createReq := mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name:      "session_create",
			Arguments: map[string]any{},
		},
	}
	createResult, err := createHandler(ctx, createReq)
	if err != nil {
		t.Fatalf("Create handler returned error: %v", err)
	}
	if createResult.IsError {
		skipIfPlaywrightNotInstalled(t, extractErrorFromResult(createResult))
		t.Fatalf("Create failed: %s", extractTextContent(createResult.Content))
	}
	sessionID := strings.TrimPrefix(extractTextContent(createResult.Content), "Created session: ")
	defer mgr.CloseSession(ctx, "", sessionID)

	// Navigate to a test page
	navReq := mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name: "navigate",
			Arguments: map[string]any{
				"sessionId": sessionID,
				"url":       "data:text/html,<h1>Screenshot Test</h1><p>Test content</p>",
			},
		},
	}
	navigateHandler(ctx, navReq)

	t.Run("captures page screenshot", func(t *testing.T) {
		req := mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name: "screenshot",
				Arguments: map[string]any{
					"sessionId": sessionID,
				},
			},
		}

		result, err := screenshotHandler(ctx, req)
		if err != nil {
			t.Fatalf("Handler returned error: %v", err)
		}

		if result.IsError {
			t.Fatalf("Expected success, got error: %s", extractTextContent(result.Content))
		}

		// Verify we got image content
		if len(result.Content) == 0 {
			t.Error("Expected content in result")
		}
	})

	t.Run("captures fullPage screenshot", func(t *testing.T) {
		req := mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name: "screenshot",
				Arguments: map[string]any{
					"sessionId": sessionID,
					"fullPage":  true,
				},
			},
		}

		result, err := screenshotHandler(ctx, req)
		if err != nil {
			t.Fatalf("Handler returned error: %v", err)
		}

		if result.IsError {
			t.Fatalf("Expected success, got error: %s", extractTextContent(result.Content))
		}
	})
}

// TestExtractTextToolIntegration tests the extract_text tool with real browser instances.
func TestExtractTextToolIntegration(t *testing.T) {
	pool, mgr := setupPoolAndManager(t)
	defer pool.Stop(context.Background())

	createHandler := SessionCreateHandler(mgr, DefaultTimeoutConfig())
	navigateHandler := NavigateHandler(mgr, DefaultTimeoutConfig(), nil)
	extractTextHandler := ExtractTextHandler(mgr, DefaultTimeoutConfig(), DefaultResponseConfig())
	ctx := context.Background()

	// Create a session
	createReq := mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name:      "session_create",
			Arguments: map[string]any{},
		},
	}
	createResult, err := createHandler(ctx, createReq)
	if err != nil {
		t.Fatalf("Create handler returned error: %v", err)
	}
	if createResult.IsError {
		skipIfPlaywrightNotInstalled(t, extractErrorFromResult(createResult))
		t.Fatalf("Create failed: %s", extractTextContent(createResult.Content))
	}
	sessionID := strings.TrimPrefix(extractTextContent(createResult.Content), "Created session: ")
	defer mgr.CloseSession(ctx, "", sessionID)

	// Navigate to a test page with structured content
	navReq := mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name: "navigate",
			Arguments: map[string]any{
				"sessionId": sessionID,
				"url":       "data:text/html,<html><body><h1>Title</h1><p>Paragraph text</p><div><span>Nested content</span></div></body></html>",
			},
		},
	}
	navigateHandler(ctx, navReq)

	t.Run("extracts text from body", func(t *testing.T) {
		req := mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name: "extract_text",
				Arguments: map[string]any{
					"sessionId": sessionID,
				},
			},
		}

		result, err := extractTextHandler(ctx, req)
		if err != nil {
			t.Fatalf("Handler returned error: %v", err)
		}

		if result.IsError {
			t.Fatalf("Expected success, got error: %s", extractTextContent(result.Content))
		}

		text := extractTextContent(result.Content)
		if !strings.Contains(text, "Title") {
			t.Errorf("Expected 'Title' in response, got: %s", text)
		}
		if !strings.Contains(text, "Paragraph text") {
			t.Errorf("Expected 'Paragraph text' in response, got: %s", text)
		}
	})

	t.Run("extracts text from specific selector", func(t *testing.T) {
		req := mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name: "extract_text",
				Arguments: map[string]any{
					"sessionId": sessionID,
					"selector":  "h1",
				},
			},
		}

		result, err := extractTextHandler(ctx, req)
		if err != nil {
			t.Fatalf("Handler returned error: %v", err)
		}

		if result.IsError {
			t.Fatalf("Expected success, got error: %s", extractTextContent(result.Content))
		}

		text := extractTextContent(result.Content)
		if !strings.Contains(text, "Title") {
			t.Errorf("Expected 'Title' in response, got: %s", text)
		}
	})

	t.Run("returns error for non-matching selector", func(t *testing.T) {
		req := mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name: "extract_text",
				Arguments: map[string]any{
					"sessionId": sessionID,
					"selector":  "#non-existent-element",
				},
			},
		}

		result, err := extractTextHandler(ctx, req)
		if err != nil {
			t.Fatalf("Handler returned error: %v", err)
		}

		if !result.IsError {
			t.Error("Expected error for non-matching selector")
		}

		text := extractTextContent(result.Content)
		if !strings.Contains(text, "[-32002]") {
			t.Errorf("Expected element not found error code, got: %s", text)
		}
	})
}

// TestGetHTMLToolIntegration tests the get_html tool with real browser instances.
func TestGetHTMLToolIntegration(t *testing.T) {
	pool, mgr := setupPoolAndManager(t)
	defer pool.Stop(context.Background())

	createHandler := SessionCreateHandler(mgr, DefaultTimeoutConfig())
	navigateHandler := NavigateHandler(mgr, DefaultTimeoutConfig(), nil)
	getHTMLHandler := GetHTMLHandler(mgr, DefaultTimeoutConfig(), DefaultResponseConfig())
	ctx := context.Background()

	// Create a session
	createReq := mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name:      "session_create",
			Arguments: map[string]any{},
		},
	}
	createResult, err := createHandler(ctx, createReq)
	if err != nil {
		t.Fatalf("Create handler returned error: %v", err)
	}
	if createResult.IsError {
		skipIfPlaywrightNotInstalled(t, extractErrorFromResult(createResult))
		t.Fatalf("Create failed: %s", extractTextContent(createResult.Content))
	}
	sessionID := strings.TrimPrefix(extractTextContent(createResult.Content), "Created session: ")
	defer mgr.CloseSession(ctx, "", sessionID)

	// Navigate to a test page
	navReq := mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name: "navigate",
			Arguments: map[string]any{
				"sessionId": sessionID,
				"url":       "data:text/html,<html><body><div id=\"content\"><p>Test paragraph</p></div></body></html>",
			},
		},
	}
	navigateHandler(ctx, navReq)

	t.Run("gets full page HTML", func(t *testing.T) {
		req := mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name: "get_html",
				Arguments: map[string]any{
					"sessionId": sessionID,
				},
			},
		}

		result, err := getHTMLHandler(ctx, req)
		if err != nil {
			t.Fatalf("Handler returned error: %v", err)
		}

		if result.IsError {
			t.Fatalf("Expected success, got error: %s", extractTextContent(result.Content))
		}

		text := extractTextContent(result.Content)
		if !strings.Contains(text, "Test paragraph") {
			t.Errorf("Expected 'Test paragraph' in HTML, got: %s", text)
		}
	})

	t.Run("gets element innerHTML", func(t *testing.T) {
		req := mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name: "get_html",
				Arguments: map[string]any{
					"sessionId": sessionID,
					"selector":  "#content",
				},
			},
		}

		result, err := getHTMLHandler(ctx, req)
		if err != nil {
			t.Fatalf("Handler returned error: %v", err)
		}

		if result.IsError {
			t.Fatalf("Expected success, got error: %s", extractTextContent(result.Content))
		}

		text := extractTextContent(result.Content)
		if !strings.Contains(text, "<p>Test paragraph</p>") {
			t.Errorf("Expected '<p>Test paragraph</p>' in innerHTML, got: %s", text)
		}
	})

	t.Run("gets element outerHTML", func(t *testing.T) {
		req := mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name: "get_html",
				Arguments: map[string]any{
					"sessionId": sessionID,
					"selector":  "#content",
					"outer":     true,
				},
			},
		}

		result, err := getHTMLHandler(ctx, req)
		if err != nil {
			t.Fatalf("Handler returned error: %v", err)
		}

		if result.IsError {
			t.Fatalf("Expected success, got error: %s", extractTextContent(result.Content))
		}

		text := extractTextContent(result.Content)
		if !strings.Contains(text, "id=\"content\"") {
			t.Errorf("Expected 'id=\"content\"' in outerHTML, got: %s", text)
		}
	})
}

// TestEvaluateToolIntegration tests the evaluate tool with real browser instances.
func TestEvaluateToolIntegration(t *testing.T) {
	pool, mgr := setupPoolAndManager(t)
	defer pool.Stop(context.Background())

	createHandler := SessionCreateHandler(mgr, DefaultTimeoutConfig())
	navigateHandler := NavigateHandler(mgr, DefaultTimeoutConfig(), nil)
	evaluateHandler := EvaluateHandler(mgr, DefaultTimeoutConfig(), DefaultResponseConfig())
	ctx := context.Background()

	// Create a session
	createReq := mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name:      "session_create",
			Arguments: map[string]any{},
		},
	}
	createResult, err := createHandler(ctx, createReq)
	if err != nil {
		t.Fatalf("Create handler returned error: %v", err)
	}
	if createResult.IsError {
		skipIfPlaywrightNotInstalled(t, extractErrorFromResult(createResult))
		t.Fatalf("Create failed: %s", extractTextContent(createResult.Content))
	}
	sessionID := strings.TrimPrefix(extractTextContent(createResult.Content), "Created session: ")
	defer mgr.CloseSession(ctx, "", sessionID)

	// Navigate to a test page
	navReq := mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name: "navigate",
			Arguments: map[string]any{
				"sessionId": sessionID,
				"url":       "data:text/html,<html><body><h1 id=\"title\">Hello World</h1></body></html>",
			},
		},
	}
	navigateHandler(ctx, navReq)

	t.Run("evaluates simple expression", func(t *testing.T) {
		req := mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name: "evaluate",
				Arguments: map[string]any{
					"sessionId":  sessionID,
					"expression": "1 + 2",
				},
			},
		}

		result, err := evaluateHandler(ctx, req)
		if err != nil {
			t.Fatalf("Handler returned error: %v", err)
		}

		if result.IsError {
			t.Fatalf("Expected success, got error: %s", extractTextContent(result.Content))
		}

		text := extractTextContent(result.Content)
		if text != "3" {
			t.Errorf("Expected '3', got: %s", text)
		}
	})

	t.Run("evaluates DOM query", func(t *testing.T) {
		req := mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name: "evaluate",
				Arguments: map[string]any{
					"sessionId":  sessionID,
					"expression": "document.getElementById('title').textContent",
				},
			},
		}

		result, err := evaluateHandler(ctx, req)
		if err != nil {
			t.Fatalf("Handler returned error: %v", err)
		}

		if result.IsError {
			t.Fatalf("Expected success, got error: %s", extractTextContent(result.Content))
		}

		text := extractTextContent(result.Content)
		if !strings.Contains(text, "Hello World") {
			t.Errorf("Expected 'Hello World' in response, got: %s", text)
		}
	})

	t.Run("returns object as JSON", func(t *testing.T) {
		req := mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name: "evaluate",
				Arguments: map[string]any{
					"sessionId":  sessionID,
					"expression": "({ name: 'test', value: 42 })",
				},
			},
		}

		result, err := evaluateHandler(ctx, req)
		if err != nil {
			t.Fatalf("Handler returned error: %v", err)
		}

		if result.IsError {
			t.Fatalf("Expected success, got error: %s", extractTextContent(result.Content))
		}

		text := extractTextContent(result.Content)
		var obj map[string]interface{}
		if err := json.Unmarshal([]byte(text), &obj); err != nil {
			t.Fatalf("Failed to parse JSON response: %v", err)
		}
		if obj["name"] != "test" || obj["value"] != float64(42) {
			t.Errorf("Unexpected object values: %v", obj)
		}
	})
}

// TestQuerySelectorToolIntegration tests the query_selector tool with real browser instances.
func TestQuerySelectorToolIntegration(t *testing.T) {
	pool, mgr := setupPoolAndManager(t)
	defer pool.Stop(context.Background())

	createHandler := SessionCreateHandler(mgr, DefaultTimeoutConfig())
	navigateHandler := NavigateHandler(mgr, DefaultTimeoutConfig(), nil)
	querySelectorHandler := QuerySelectorHandler(mgr, DefaultTimeoutConfig(), DefaultResponseConfig())
	ctx := context.Background()

	// Create a session
	createReq := mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name:      "session_create",
			Arguments: map[string]any{},
		},
	}
	createResult, err := createHandler(ctx, createReq)
	if err != nil {
		t.Fatalf("Create handler returned error: %v", err)
	}
	if createResult.IsError {
		skipIfPlaywrightNotInstalled(t, extractErrorFromResult(createResult))
		t.Fatalf("Create failed: %s", extractTextContent(createResult.Content))
	}
	sessionID := strings.TrimPrefix(extractTextContent(createResult.Content), "Created session: ")
	defer mgr.CloseSession(ctx, "", sessionID)

	// Navigate to a test page with multiple elements
	navReq := mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name: "navigate",
			Arguments: map[string]any{
				"sessionId": sessionID,
				"url":       "data:text/html,<html><body><button id=\"btn1\" class=\"primary\">Submit</button><button id=\"btn2\" class=\"secondary\">Cancel</button></body></html>",
			},
		},
	}
	navigateHandler(ctx, navReq)

	t.Run("queries single element", func(t *testing.T) {
		req := mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name: "query_selector",
				Arguments: map[string]any{
					"sessionId": sessionID,
					"selector":  "#btn1",
				},
			},
		}

		result, err := querySelectorHandler(ctx, req)
		if err != nil {
			t.Fatalf("Handler returned error: %v", err)
		}

		if result.IsError {
			t.Fatalf("Expected success, got error: %s", extractTextContent(result.Content))
		}

		text := extractTextContent(result.Content)
		var info ElementInfo
		if err := json.Unmarshal([]byte(text), &info); err != nil {
			t.Fatalf("Failed to parse response: %v", err)
		}

		if info.Tag != "button" {
			t.Errorf("Expected tag 'button', got '%s'", info.Tag)
		}
		if info.Attributes["id"] != "btn1" {
			t.Errorf("Expected id 'btn1', got '%s'", info.Attributes["id"])
		}
	})

	t.Run("queries all matching elements", func(t *testing.T) {
		req := mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name: "query_selector",
				Arguments: map[string]any{
					"sessionId": sessionID,
					"selector":  "button",
					"all":       true,
				},
			},
		}

		result, err := querySelectorHandler(ctx, req)
		if err != nil {
			t.Fatalf("Handler returned error: %v", err)
		}

		if result.IsError {
			t.Fatalf("Expected success, got error: %s", extractTextContent(result.Content))
		}

		text := extractTextContent(result.Content)
		var infos []ElementInfo
		if err := json.Unmarshal([]byte(text), &infos); err != nil {
			t.Fatalf("Failed to parse response: %v", err)
		}

		if len(infos) != 2 {
			t.Errorf("Expected 2 buttons, got %d", len(infos))
		}
	})

	t.Run("returns error for non-matching selector", func(t *testing.T) {
		req := mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name: "query_selector",
				Arguments: map[string]any{
					"sessionId": sessionID,
					"selector":  "#non-existent",
				},
			},
		}

		result, err := querySelectorHandler(ctx, req)
		if err != nil {
			t.Fatalf("Handler returned error: %v", err)
		}

		if !result.IsError {
			t.Error("Expected error for non-matching selector")
		}

		text := extractTextContent(result.Content)
		if !strings.Contains(text, "[-32002]") {
			t.Errorf("Expected element not found error code, got: %s", text)
		}
	})
}

// TestGetAccessibilityTreeToolIntegration tests the get_accessibility_tree tool with real browser instances.
func TestGetAccessibilityTreeToolIntegration(t *testing.T) {
	pool, mgr := setupPoolAndManager(t)
	defer pool.Stop(context.Background())

	createHandler := SessionCreateHandler(mgr, DefaultTimeoutConfig())
	navigateHandler := NavigateHandler(mgr, DefaultTimeoutConfig(), nil)
	accessibilityHandler := GetAccessibilityTreeHandler(mgr, DefaultTimeoutConfig(), DefaultResponseConfig())
	ctx := context.Background()

	// Create a session
	createReq := mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name:      "session_create",
			Arguments: map[string]any{},
		},
	}
	createResult, err := createHandler(ctx, createReq)
	if err != nil {
		t.Fatalf("Create handler returned error: %v", err)
	}
	if createResult.IsError {
		skipIfPlaywrightNotInstalled(t, extractErrorFromResult(createResult))
		t.Fatalf("Create failed: %s", extractTextContent(createResult.Content))
	}
	sessionID := strings.TrimPrefix(extractTextContent(createResult.Content), "Created session: ")
	defer mgr.CloseSession(ctx, "", sessionID)

	// Navigate to a test page with accessible elements
	ts := startTestHTTPServer(t)
	navReq := mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name: "navigate",
			Arguments: map[string]any{
				"sessionId": sessionID,
				"url":       ts.URL + "/accessible",
			},
		},
	}
	navigateHandler(ctx, navReq)

	t.Run("gets accessibility tree", func(t *testing.T) {
		req := mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name: "get_accessibility_tree",
				Arguments: map[string]any{
					"sessionId": sessionID,
				},
			},
		}

		result, err := accessibilityHandler(ctx, req)
		if err != nil {
			t.Fatalf("Handler returned error: %v", err)
		}

		if result.IsError {
			t.Fatalf("Expected success, got error: %s", extractTextContent(result.Content))
		}

		text := extractTextContent(result.Content)

		// The snapshot is YAML, not JSON
		var asJSON any
		if json.Unmarshal([]byte(text), &asJSON) == nil {
			t.Fatalf("Expected a YAML snapshot, got JSON: %s", text)
		}

		// Elements appear as "- <role> "<name>" [ref=e<n>]" lines
		lineRe := regexp.MustCompile(`(?m)^\s*- (\w+) "([^"]*)"(?: \[[^\]]+\])* \[ref=e\d+\]`)
		for _, want := range []struct{ role, name string }{
			{"button", "Submit Form"},
			{"link", "Home"},
			{"heading", "Title"},
		} {
			found := false
			for _, m := range lineRe.FindAllStringSubmatch(text, -1) {
				if m[1] == want.role && m[2] == want.name {
					found = true
				}
			}
			if !found {
				t.Errorf("Expected a line '- %s \"%s\" [ref=e<n>]' in snapshot:\n%s", want.role, want.name, text)
			}
		}
	})

	t.Run("scoped selector", func(t *testing.T) {
		req := mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name: "get_accessibility_tree",
				Arguments: map[string]any{
					"sessionId": sessionID,
					"selector":  "main",
				},
			},
		}
		result, err := accessibilityHandler(ctx, req)
		if err != nil || result.IsError {
			t.Fatalf("Expected success, got: %v %s", err, extractTextContent(result.Content))
		}
		text := extractTextContent(result.Content)
		if !strings.Contains(text, `button "Submit Form"`) {
			t.Errorf("Expected the button inside main, got:\n%s", text)
		}
		if strings.Contains(text, `link "Home"`) {
			t.Errorf("Expected the nav link outside main to be left out, got:\n%s", text)
		}
	})

	t.Run("scoped selector missing", func(t *testing.T) {
		req := mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name: "get_accessibility_tree",
				Arguments: map[string]any{
					"sessionId": sessionID,
					"selector":  "#does-not-exist",
				},
			},
		}
		start := time.Now()
		result, err := accessibilityHandler(ctx, req)
		elapsed := time.Since(start)
		if err != nil {
			t.Fatalf("Handler returned error: %v", err)
		}
		text := extractTextContent(result.Content)
		if !result.IsError || !strings.HasPrefix(text, "[-32002]") {
			t.Fatalf("Expected [-32002] error, got: %s", text)
		}
		if elapsed > 2*time.Second {
			t.Errorf("Expected an immediate failure, took %v", elapsed)
		}
	})
}

// TestGetConsoleLogsToolIntegration tests the get_console_logs tool with real browser instances.
func TestGetConsoleLogsToolIntegration(t *testing.T) {
	pool, mgr := setupPoolAndManager(t)
	defer pool.Stop(context.Background())

	createHandler := SessionCreateHandler(mgr, DefaultTimeoutConfig())
	navigateHandler := NavigateHandler(mgr, DefaultTimeoutConfig(), nil)
	evaluateHandler := EvaluateHandler(mgr, DefaultTimeoutConfig(), DefaultResponseConfig())
	consoleLogsHandler := GetConsoleLogsHandler(mgr, DefaultTimeoutConfig(), DefaultResponseConfig())
	ctx := context.Background()

	// Create a session
	createReq := mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name:      "session_create",
			Arguments: map[string]any{},
		},
	}
	createResult, err := createHandler(ctx, createReq)
	if err != nil {
		t.Fatalf("Create handler returned error: %v", err)
	}
	if createResult.IsError {
		skipIfPlaywrightNotInstalled(t, extractErrorFromResult(createResult))
		t.Fatalf("Create failed: %s", extractTextContent(createResult.Content))
	}
	sessionID := strings.TrimPrefix(extractTextContent(createResult.Content), "Created session: ")
	defer mgr.CloseSession(ctx, "", sessionID)

	// Navigate to a test page
	navReq := mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name: "navigate",
			Arguments: map[string]any{
				"sessionId": sessionID,
				"url":       "about:blank",
			},
		},
	}
	navigateHandler(ctx, navReq)

	// Generate console logs using evaluate
	evalReq := mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name: "evaluate",
			Arguments: map[string]any{
				"sessionId":  sessionID,
				"expression": "console.log('test log'); console.warn('test warning'); console.error('test error'); true",
			},
		},
	}
	evaluateHandler(ctx, evalReq)

	t.Run("retrieves console logs", func(t *testing.T) {
		req := mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name: "get_console_logs",
				Arguments: map[string]any{
					"sessionId": sessionID,
				},
			},
		}

		result, err := consoleLogsHandler(ctx, req)
		if err != nil {
			t.Fatalf("Handler returned error: %v", err)
		}

		if result.IsError {
			t.Fatalf("Expected success, got error: %s", extractTextContent(result.Content))
		}

		text := extractTextContent(result.Content)
		var logs []session.ConsoleLogEntry
		if err := json.Unmarshal([]byte(text), &logs); err != nil {
			t.Fatalf("Failed to parse console logs: %v", err)
		}

		// Verify we captured logs
		if len(logs) < 3 {
			t.Errorf("Expected at least 3 log entries, got %d", len(logs))
		}
	})

	t.Run("filters by level", func(t *testing.T) {
		req := mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name: "get_console_logs",
				Arguments: map[string]any{
					"sessionId": sessionID,
					"level":     "error",
				},
			},
		}

		result, err := consoleLogsHandler(ctx, req)
		if err != nil {
			t.Fatalf("Handler returned error: %v", err)
		}

		if result.IsError {
			t.Fatalf("Expected success, got error: %s", extractTextContent(result.Content))
		}

		text := extractTextContent(result.Content)
		var logs []session.ConsoleLogEntry
		if err := json.Unmarshal([]byte(text), &logs); err != nil {
			t.Fatalf("Failed to parse console logs: %v", err)
		}

		// All returned logs should be error level
		for _, log := range logs {
			if log.Level != session.ConsoleLogLevelError {
				t.Errorf("Expected error level, got %s", log.Level)
			}
		}
	})
}

// TestClickToolIntegration tests the click tool with real browser instances.
func TestClickToolIntegration(t *testing.T) {
	pool, mgr := setupPoolAndManager(t)
	defer pool.Stop(context.Background())

	createHandler := SessionCreateHandler(mgr, DefaultTimeoutConfig())
	navigateHandler := NavigateHandler(mgr, DefaultTimeoutConfig(), nil)
	clickHandler := ClickHandler(mgr, DefaultTimeoutConfig(), nil)
	ctx := context.Background()

	// Create a session
	createReq := mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name:      "session_create",
			Arguments: map[string]any{},
		},
	}
	createResult, err := createHandler(ctx, createReq)
	if err != nil {
		t.Fatalf("Create handler returned error: %v", err)
	}
	if createResult.IsError {
		skipIfPlaywrightNotInstalled(t, extractErrorFromResult(createResult))
		t.Fatalf("Create failed: %s", extractTextContent(createResult.Content))
	}
	sessionID := strings.TrimPrefix(extractTextContent(createResult.Content), "Created session: ")
	defer mgr.CloseSession(ctx, "", sessionID)

	// Navigate to a page with a clickable button
	testPage := `data:text/html,<html><body><button id="btn" onclick="this.textContent='clicked'">Click me</button></body></html>`
	navReq := mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name: "navigate",
			Arguments: map[string]any{
				"sessionId": sessionID,
				"url":       testPage,
			},
		},
	}
	navResult, err := navigateHandler(ctx, navReq)
	if err != nil || navResult.IsError {
		t.Fatalf("Navigation failed: %v", extractTextContent(navResult.Content))
	}

	t.Run("clicks element successfully", func(t *testing.T) {
		req := mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name: "click",
				Arguments: map[string]any{
					"sessionId": sessionID,
					"selector":  "#btn",
				},
			},
		}

		result, err := clickHandler(ctx, req)
		if err != nil {
			t.Fatalf("Handler returned error: %v", err)
		}

		if result.IsError {
			t.Fatalf("Expected success, got error: %s", extractTextContent(result.Content))
		}

		text := extractTextContent(result.Content)
		if !strings.Contains(text, "Clicked element") {
			t.Errorf("Expected click success message, got: %s", text)
		}
	})

	t.Run("returns error for non-existent element", func(t *testing.T) {
		req := mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name: "click",
				Arguments: map[string]any{
					"sessionId": sessionID,
					"selector":  "#non-existent",
					"timeout":   float64(1000),
				},
			},
		}

		result, err := clickHandler(ctx, req)
		if err != nil {
			t.Fatalf("Handler returned error: %v", err)
		}

		if !result.IsError {
			t.Error("Expected error for non-existent element")
		}
	})

	t.Run("returns error for invalid session", func(t *testing.T) {
		req := mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name: "click",
				Arguments: map[string]any{
					"sessionId": "sess-invalid",
					"selector":  "#btn",
				},
			},
		}

		result, err := clickHandler(ctx, req)
		if err != nil {
			t.Fatalf("Handler returned error: %v", err)
		}

		if !result.IsError {
			t.Error("Expected error for invalid session")
		}

		text := extractTextContent(result.Content)
		if !strings.Contains(text, "[-32001]") {
			t.Errorf("Expected session not found error code, got: %s", text)
		}
	})
}

// TestTypeToolIntegration tests the type tool with real browser instances.
func TestTypeToolIntegration(t *testing.T) {
	pool, mgr := setupPoolAndManager(t)
	defer pool.Stop(context.Background())

	createHandler := SessionCreateHandler(mgr, DefaultTimeoutConfig())
	navigateHandler := NavigateHandler(mgr, DefaultTimeoutConfig(), nil)
	typeHandler := TypeHandler(mgr, DefaultTimeoutConfig(), nil)
	ctx := context.Background()

	// Create a session
	createReq := mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name:      "session_create",
			Arguments: map[string]any{},
		},
	}
	createResult, err := createHandler(ctx, createReq)
	if err != nil {
		t.Fatalf("Create handler returned error: %v", err)
	}
	if createResult.IsError {
		skipIfPlaywrightNotInstalled(t, extractErrorFromResult(createResult))
		t.Fatalf("Create failed: %s", extractTextContent(createResult.Content))
	}
	sessionID := strings.TrimPrefix(extractTextContent(createResult.Content), "Created session: ")
	defer mgr.CloseSession(ctx, "", sessionID)

	// Navigate to a page with an input field
	testPage := `data:text/html,<html><body><input id="input" type="text" /></body></html>`
	navReq := mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name: "navigate",
			Arguments: map[string]any{
				"sessionId": sessionID,
				"url":       testPage,
			},
		},
	}
	navResult, err := navigateHandler(ctx, navReq)
	if err != nil || navResult.IsError {
		t.Fatalf("Navigation failed: %v", extractTextContent(navResult.Content))
	}

	t.Run("types text into input", func(t *testing.T) {
		req := mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name: "type",
				Arguments: map[string]any{
					"sessionId": sessionID,
					"selector":  "#input",
					"text":      "Hello World",
				},
			},
		}

		result, err := typeHandler(ctx, req)
		if err != nil {
			t.Fatalf("Handler returned error: %v", err)
		}

		if result.IsError {
			t.Fatalf("Expected success, got error: %s", extractTextContent(result.Content))
		}

		text := extractTextContent(result.Content)
		if !strings.Contains(text, "Typed text into element") {
			t.Errorf("Expected type success message, got: %s", text)
		}
	})

	t.Run("returns error for invalid session", func(t *testing.T) {
		req := mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name: "type",
				Arguments: map[string]any{
					"sessionId": "sess-invalid",
					"selector":  "#input",
					"text":      "test",
				},
			},
		}

		result, err := typeHandler(ctx, req)
		if err != nil {
			t.Fatalf("Handler returned error: %v", err)
		}

		if !result.IsError {
			t.Error("Expected error for invalid session")
		}
	})
}

// TestFillToolIntegration tests the fill tool with real browser instances.
func TestFillToolIntegration(t *testing.T) {
	pool, mgr := setupPoolAndManager(t)
	defer pool.Stop(context.Background())

	createHandler := SessionCreateHandler(mgr, DefaultTimeoutConfig())
	navigateHandler := NavigateHandler(mgr, DefaultTimeoutConfig(), nil)
	fillHandler := FillHandler(mgr, DefaultTimeoutConfig(), nil)
	ctx := context.Background()

	// Create a session
	createReq := mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name:      "session_create",
			Arguments: map[string]any{},
		},
	}
	createResult, err := createHandler(ctx, createReq)
	if err != nil {
		t.Fatalf("Create handler returned error: %v", err)
	}
	if createResult.IsError {
		skipIfPlaywrightNotInstalled(t, extractErrorFromResult(createResult))
		t.Fatalf("Create failed: %s", extractTextContent(createResult.Content))
	}
	sessionID := strings.TrimPrefix(extractTextContent(createResult.Content), "Created session: ")
	defer mgr.CloseSession(ctx, "", sessionID)

	// Navigate to a page with an input field
	testPage := `data:text/html,<html><body><input id="input" type="text" value="existing" /></body></html>`
	navReq := mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name: "navigate",
			Arguments: map[string]any{
				"sessionId": sessionID,
				"url":       testPage,
			},
		},
	}
	navResult, err := navigateHandler(ctx, navReq)
	if err != nil || navResult.IsError {
		t.Fatalf("Navigation failed: %v", extractTextContent(navResult.Content))
	}

	t.Run("fills input with text", func(t *testing.T) {
		req := mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name: "fill",
				Arguments: map[string]any{
					"sessionId": sessionID,
					"selector":  "#input",
					"value":     "New Value",
				},
			},
		}

		result, err := fillHandler(ctx, req)
		if err != nil {
			t.Fatalf("Handler returned error: %v", err)
		}

		if result.IsError {
			t.Fatalf("Expected success, got error: %s", extractTextContent(result.Content))
		}

		text := extractTextContent(result.Content)
		if !strings.Contains(text, "Filled element") {
			t.Errorf("Expected fill success message, got: %s", text)
		}
	})

	t.Run("returns error for invalid session", func(t *testing.T) {
		req := mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name: "fill",
				Arguments: map[string]any{
					"sessionId": "sess-invalid",
					"selector":  "#input",
					"value":     "test",
				},
			},
		}

		result, err := fillHandler(ctx, req)
		if err != nil {
			t.Fatalf("Handler returned error: %v", err)
		}

		if !result.IsError {
			t.Error("Expected error for invalid session")
		}
	})
}

// TestSelectOptionToolIntegration tests the select_option tool with real browser instances.
func TestSelectOptionToolIntegration(t *testing.T) {
	pool, mgr := setupPoolAndManager(t)
	defer pool.Stop(context.Background())

	createHandler := SessionCreateHandler(mgr, DefaultTimeoutConfig())
	navigateHandler := NavigateHandler(mgr, DefaultTimeoutConfig(), nil)
	selectHandler := SelectOptionHandler(mgr, DefaultTimeoutConfig(), nil)
	ctx := context.Background()

	// Create a session
	createReq := mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name:      "session_create",
			Arguments: map[string]any{},
		},
	}
	createResult, err := createHandler(ctx, createReq)
	if err != nil {
		t.Fatalf("Create handler returned error: %v", err)
	}
	if createResult.IsError {
		skipIfPlaywrightNotInstalled(t, extractErrorFromResult(createResult))
		t.Fatalf("Create failed: %s", extractTextContent(createResult.Content))
	}
	sessionID := strings.TrimPrefix(extractTextContent(createResult.Content), "Created session: ")
	defer mgr.CloseSession(ctx, "", sessionID)

	// Navigate to a page with a select dropdown
	testPage := `data:text/html,<html><body><select id="select"><option value="a">Option A</option><option value="b">Option B</option><option value="c">Option C</option></select></body></html>`
	navReq := mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name: "navigate",
			Arguments: map[string]any{
				"sessionId": sessionID,
				"url":       testPage,
			},
		},
	}
	navResult, err := navigateHandler(ctx, navReq)
	if err != nil || navResult.IsError {
		t.Fatalf("Navigation failed: %v", extractTextContent(navResult.Content))
	}

	t.Run("selects option by value", func(t *testing.T) {
		req := mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name: "select_option",
				Arguments: map[string]any{
					"sessionId": sessionID,
					"selector":  "#select",
					"value":     "b",
				},
			},
		}

		result, err := selectHandler(ctx, req)
		if err != nil {
			t.Fatalf("Handler returned error: %v", err)
		}

		if result.IsError {
			t.Fatalf("Expected success, got error: %s", extractTextContent(result.Content))
		}

		text := extractTextContent(result.Content)
		if !strings.Contains(text, "Selected option") {
			t.Errorf("Expected select success message, got: %s", text)
		}
	})

	t.Run("returns error for invalid session", func(t *testing.T) {
		req := mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name: "select_option",
				Arguments: map[string]any{
					"sessionId": "sess-invalid",
					"selector":  "#select",
					"value":     "a",
				},
			},
		}

		result, err := selectHandler(ctx, req)
		if err != nil {
			t.Fatalf("Handler returned error: %v", err)
		}

		if !result.IsError {
			t.Error("Expected error for invalid session")
		}
	})
}

// TestHoverToolIntegration tests the hover tool with real browser instances.
func TestHoverToolIntegration(t *testing.T) {
	pool, mgr := setupPoolAndManager(t)
	defer pool.Stop(context.Background())

	createHandler := SessionCreateHandler(mgr, DefaultTimeoutConfig())
	navigateHandler := NavigateHandler(mgr, DefaultTimeoutConfig(), nil)
	hoverHandler := HoverHandler(mgr, DefaultTimeoutConfig(), nil)
	ctx := context.Background()

	// Create a session
	createReq := mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name:      "session_create",
			Arguments: map[string]any{},
		},
	}
	createResult, err := createHandler(ctx, createReq)
	if err != nil {
		t.Fatalf("Create handler returned error: %v", err)
	}
	if createResult.IsError {
		skipIfPlaywrightNotInstalled(t, extractErrorFromResult(createResult))
		t.Fatalf("Create failed: %s", extractTextContent(createResult.Content))
	}
	sessionID := strings.TrimPrefix(extractTextContent(createResult.Content), "Created session: ")
	defer mgr.CloseSession(ctx, "", sessionID)

	// Navigate to a page with a hoverable element
	testPage := `data:text/html,<html><body><div id="hover-target" style="width:100px;height:100px;background:blue;">Hover me</div></body></html>`
	navReq := mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name: "navigate",
			Arguments: map[string]any{
				"sessionId": sessionID,
				"url":       testPage,
			},
		},
	}
	navResult, err := navigateHandler(ctx, navReq)
	if err != nil || navResult.IsError {
		t.Fatalf("Navigation failed: %v", extractTextContent(navResult.Content))
	}

	t.Run("hovers over element successfully", func(t *testing.T) {
		req := mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name: "hover",
				Arguments: map[string]any{
					"sessionId": sessionID,
					"selector":  "#hover-target",
				},
			},
		}

		result, err := hoverHandler(ctx, req)
		if err != nil {
			t.Fatalf("Handler returned error: %v", err)
		}

		if result.IsError {
			t.Fatalf("Expected success, got error: %s", extractTextContent(result.Content))
		}

		text := extractTextContent(result.Content)
		if !strings.Contains(text, "Hovered over element") {
			t.Errorf("Expected hover success message, got: %s", text)
		}
	})

	t.Run("returns error for invalid session", func(t *testing.T) {
		req := mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name: "hover",
				Arguments: map[string]any{
					"sessionId": "sess-invalid",
					"selector":  "#hover-target",
				},
			},
		}

		result, err := hoverHandler(ctx, req)
		if err != nil {
			t.Fatalf("Handler returned error: %v", err)
		}

		if !result.IsError {
			t.Error("Expected error for invalid session")
		}
	})
}

// TestPressKeyToolIntegration tests the press_key tool with real browser instances.
func TestPressKeyToolIntegration(t *testing.T) {
	pool, mgr := setupPoolAndManager(t)
	defer pool.Stop(context.Background())

	createHandler := SessionCreateHandler(mgr, DefaultTimeoutConfig())
	navigateHandler := NavigateHandler(mgr, DefaultTimeoutConfig(), nil)
	pressKeyHandler := PressKeyHandler(mgr, DefaultTimeoutConfig(), nil)
	ctx := context.Background()

	// Create a session
	createReq := mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name:      "session_create",
			Arguments: map[string]any{},
		},
	}
	createResult, err := createHandler(ctx, createReq)
	if err != nil {
		t.Fatalf("Create handler returned error: %v", err)
	}
	if createResult.IsError {
		skipIfPlaywrightNotInstalled(t, extractErrorFromResult(createResult))
		t.Fatalf("Create failed: %s", extractTextContent(createResult.Content))
	}
	sessionID := strings.TrimPrefix(extractTextContent(createResult.Content), "Created session: ")
	defer mgr.CloseSession(ctx, "", sessionID)

	// Navigate to a page with a text area
	testPage := `data:text/html,<html><body><textarea id="textarea"></textarea></body></html>`
	navReq := mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name: "navigate",
			Arguments: map[string]any{
				"sessionId": sessionID,
				"url":       testPage,
			},
		},
	}
	navResult, err := navigateHandler(ctx, navReq)
	if err != nil || navResult.IsError {
		t.Fatalf("Navigation failed: %v", extractTextContent(navResult.Content))
	}

	t.Run("presses key successfully", func(t *testing.T) {
		req := mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name: "press_key",
				Arguments: map[string]any{
					"sessionId": sessionID,
					"key":       "Tab",
				},
			},
		}

		result, err := pressKeyHandler(ctx, req)
		if err != nil {
			t.Fatalf("Handler returned error: %v", err)
		}

		if result.IsError {
			t.Fatalf("Expected success, got error: %s", extractTextContent(result.Content))
		}

		text := extractTextContent(result.Content)
		if !strings.Contains(text, "Pressed key") {
			t.Errorf("Expected press key success message, got: %s", text)
		}
	})

	t.Run("presses key with modifiers", func(t *testing.T) {
		req := mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name: "press_key",
				Arguments: map[string]any{
					"sessionId": sessionID,
					"key":       "a",
					"modifiers": []any{"Control"},
				},
			},
		}

		result, err := pressKeyHandler(ctx, req)
		if err != nil {
			t.Fatalf("Handler returned error: %v", err)
		}

		if result.IsError {
			t.Fatalf("Expected success, got error: %s", extractTextContent(result.Content))
		}

		text := extractTextContent(result.Content)
		if !strings.Contains(text, "Pressed key") {
			t.Errorf("Expected press key success message, got: %s", text)
		}
	})

	t.Run("returns error for invalid session", func(t *testing.T) {
		req := mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name: "press_key",
				Arguments: map[string]any{
					"sessionId": "sess-invalid",
					"key":       "Enter",
				},
			},
		}

		result, err := pressKeyHandler(ctx, req)
		if err != nil {
			t.Fatalf("Handler returned error: %v", err)
		}

		if !result.IsError {
			t.Error("Expected error for invalid session")
		}
	})
}

// TestGetNetworkLogsToolIntegration tests the get_network_logs tool with real browser instances.
func TestGetNetworkLogsToolIntegration(t *testing.T) {
	pool, mgr := setupPoolAndManager(t)
	defer pool.Stop(context.Background())

	createHandler := SessionCreateHandler(mgr, DefaultTimeoutConfig())
	navigateHandler := NavigateHandler(mgr, DefaultTimeoutConfig(), nil)
	networkLogsHandler := GetNetworkLogsHandler(mgr, DefaultTimeoutConfig(), DefaultResponseConfig())
	closeHandler := SessionCloseHandler(mgr, DefaultTimeoutConfig())
	ctx := context.Background()

	// Create a session
	createReq := mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name:      "session_create",
			Arguments: map[string]any{},
		},
	}
	createResult, err := createHandler(ctx, createReq)
	if err != nil {
		t.Fatalf("Create handler returned error: %v", err)
	}
	if createResult.IsError {
		skipIfPlaywrightNotInstalled(t, extractErrorFromResult(createResult))
		t.Fatalf("Create failed: %s", extractTextContent(createResult.Content))
	}
	sessionID := strings.TrimPrefix(extractTextContent(createResult.Content), "Created session: ")
	defer mgr.CloseSession(ctx, "", sessionID)

	t.Run("retrieves network logs after navigation", func(t *testing.T) {
		// Navigate to a data URL - this should generate at least one network request
		navReq := mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name: "navigate",
				Arguments: map[string]any{
					"sessionId": sessionID,
					"url":       "data:text/html,<html><body><h1>Test Page</h1></body></html>",
				},
			},
		}
		navResult, err := navigateHandler(ctx, navReq)
		if err != nil || navResult.IsError {
			t.Fatalf("Navigation failed: %v", extractTextContent(navResult.Content))
		}

		// Get network logs
		req := mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name: "get_network_logs",
				Arguments: map[string]any{
					"sessionId": sessionID,
				},
			},
		}

		result, err := networkLogsHandler(ctx, req)
		if err != nil {
			t.Fatalf("Handler returned error: %v", err)
		}

		if result.IsError {
			t.Fatalf("Expected success, got error: %s", extractTextContent(result.Content))
		}

		text := extractTextContent(result.Content)
		var logs []networkLogOutput
		if err := json.Unmarshal([]byte(text), &logs); err != nil {
			t.Fatalf("Failed to parse network logs: %v", err)
		}

		// Data URLs may or may not generate network entries depending on the browser,
		// but the response should be valid JSON array
		_ = logs
	})

	t.Run("retrieves network logs after multiple navigations", func(t *testing.T) {
		// Navigate to first page
		navReq1 := mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name: "navigate",
				Arguments: map[string]any{
					"sessionId": sessionID,
					"url":       "data:text/html,<html><body><h1>Page 1</h1></body></html>",
				},
			},
		}
		navigateHandler(ctx, navReq1)

		// Navigate to second page
		navReq2 := mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name: "navigate",
				Arguments: map[string]any{
					"sessionId": sessionID,
					"url":       "data:text/html,<html><body><h1>Page 2</h1></body></html>",
				},
			},
		}
		navigateHandler(ctx, navReq2)

		// Get network logs
		req := mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name: "get_network_logs",
				Arguments: map[string]any{
					"sessionId": sessionID,
				},
			},
		}

		result, err := networkLogsHandler(ctx, req)
		if err != nil {
			t.Fatalf("Handler returned error: %v", err)
		}

		if result.IsError {
			t.Fatalf("Expected success, got error: %s", extractTextContent(result.Content))
		}

		text := extractTextContent(result.Content)
		var logs []networkLogOutput
		if err := json.Unmarshal([]byte(text), &logs); err != nil {
			t.Fatalf("Failed to parse network logs: %v", err)
		}

		// Response should be valid JSON array
		_ = logs
	})

	t.Run("respects limit parameter", func(t *testing.T) {
		req := mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name: "get_network_logs",
				Arguments: map[string]any{
					"sessionId": sessionID,
					"limit":     float64(1),
				},
			},
		}

		result, err := networkLogsHandler(ctx, req)
		if err != nil {
			t.Fatalf("Handler returned error: %v", err)
		}

		if result.IsError {
			t.Fatalf("Expected success, got error: %s", extractTextContent(result.Content))
		}

		text := extractTextContent(result.Content)
		var logs []networkLogOutput
		if err := json.Unmarshal([]byte(text), &logs); err != nil {
			t.Fatalf("Failed to parse network logs: %v", err)
		}

		if len(logs) > 1 {
			t.Errorf("Expected at most 1 entry with limit=1, got %d", len(logs))
		}
	})

	t.Run("filters by URL pattern", func(t *testing.T) {
		req := mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name: "get_network_logs",
				Arguments: map[string]any{
					"sessionId": sessionID,
					"filter": map[string]any{
						"urlPattern": "data:",
					},
				},
			},
		}

		result, err := networkLogsHandler(ctx, req)
		if err != nil {
			t.Fatalf("Handler returned error: %v", err)
		}

		if result.IsError {
			t.Fatalf("Expected success, got error: %s", extractTextContent(result.Content))
		}

		text := extractTextContent(result.Content)
		var logs []networkLogOutput
		if err := json.Unmarshal([]byte(text), &logs); err != nil {
			t.Fatalf("Failed to parse network logs: %v", err)
		}

		// All returned entries should match the URL pattern
		for _, log := range logs {
			if !strings.Contains(log.URL, "data:") {
				t.Errorf("URL pattern filter failed: %s doesn't match 'data:'", log.URL)
			}
		}
	})

	t.Run("filters by status code", func(t *testing.T) {
		req := mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name: "get_network_logs",
				Arguments: map[string]any{
					"sessionId": sessionID,
					"filter": map[string]any{
						"statusMin": float64(200),
						"statusMax": float64(299),
					},
				},
			},
		}

		result, err := networkLogsHandler(ctx, req)
		if err != nil {
			t.Fatalf("Handler returned error: %v", err)
		}

		if result.IsError {
			t.Fatalf("Expected success, got error: %s", extractTextContent(result.Content))
		}

		text := extractTextContent(result.Content)
		var logs []networkLogOutput
		if err := json.Unmarshal([]byte(text), &logs); err != nil {
			t.Fatalf("Failed to parse network logs: %v", err)
		}

		// All returned entries should have 2xx status
		for _, log := range logs {
			if log.Status < 200 || log.Status > 299 {
				t.Errorf("Status filter failed: status %d not in range 200-299", log.Status)
			}
		}
	})

	t.Run("returns error for invalid regex pattern", func(t *testing.T) {
		req := mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name: "get_network_logs",
				Arguments: map[string]any{
					"sessionId": sessionID,
					"filter": map[string]any{
						"urlPattern": "[invalid(",
					},
				},
			},
		}

		result, err := networkLogsHandler(ctx, req)
		if err != nil {
			t.Fatalf("Handler returned error: %v", err)
		}

		if !result.IsError {
			t.Error("Expected error for invalid regex pattern")
		}

		text := extractTextContent(result.Content)
		if !strings.Contains(text, "invalid") && !strings.Contains(text, "regex") {
			t.Errorf("Expected invalid regex error message, got: %s", text)
		}
	})

	t.Run("returns error for non-existent session", func(t *testing.T) {
		req := mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name: "get_network_logs",
				Arguments: map[string]any{
					"sessionId": "sess-non-existent",
				},
			},
		}

		result, err := networkLogsHandler(ctx, req)
		if err != nil {
			t.Fatalf("Handler returned error: %v", err)
		}

		if !result.IsError {
			t.Error("Expected error for non-existent session")
		}

		text := extractTextContent(result.Content)
		if !strings.Contains(text, "[-32001]") {
			t.Errorf("Expected session not found error code, got: %s", text)
		}
	})

	t.Run("returns error after session is closed", func(t *testing.T) {
		// Create a new session specifically for this test
		createReq2 := mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name:      "session_create",
				Arguments: map[string]any{},
			},
		}
		createResult2, err := createHandler(ctx, createReq2)
		if err != nil || createResult2.IsError {
			t.Fatalf("Create second session failed")
		}
		sessionID2 := strings.TrimPrefix(extractTextContent(createResult2.Content), "Created session: ")

		// Close the session
		closeReq := mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name: "session_close",
				Arguments: map[string]any{
					"sessionId": sessionID2,
				},
			},
		}
		closeResult, err := closeHandler(ctx, closeReq)
		if err != nil || closeResult.IsError {
			t.Fatalf("Close session failed")
		}

		// Try to get network logs from closed session
		req := mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name: "get_network_logs",
				Arguments: map[string]any{
					"sessionId": sessionID2,
				},
			},
		}

		result, err := networkLogsHandler(ctx, req)
		if err != nil {
			t.Fatalf("Handler returned error: %v", err)
		}

		if !result.IsError {
			t.Error("Expected error for closed session")
		}

		text := extractTextContent(result.Content)
		if !strings.Contains(text, "[-32001]") && !strings.Contains(text, "not found") {
			t.Errorf("Expected session not found error, got: %s", text)
		}
	})
}

// TestGetNetworkLogsWithRealNetworkTrafficIntegration tests network logging with actual HTTP requests.
func TestGetNetworkLogsWithRealNetworkTrafficIntegration(t *testing.T) {
	pool, mgr := setupPoolAndManager(t)
	defer pool.Stop(context.Background())

	createHandler := SessionCreateHandler(mgr, DefaultTimeoutConfig())
	navigateHandler := NavigateHandler(mgr, DefaultTimeoutConfig(), nil)
	networkLogsHandler := GetNetworkLogsHandler(mgr, DefaultTimeoutConfig(), DefaultResponseConfig())
	ctx := context.Background()

	// Create a session
	createReq := mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name:      "session_create",
			Arguments: map[string]any{},
		},
	}
	createResult, err := createHandler(ctx, createReq)
	if err != nil {
		t.Fatalf("Create handler returned error: %v", err)
	}
	if createResult.IsError {
		skipIfPlaywrightNotInstalled(t, extractErrorFromResult(createResult))
		t.Fatalf("Create failed: %s", extractTextContent(createResult.Content))
	}
	sessionID := strings.TrimPrefix(extractTextContent(createResult.Content), "Created session: ")
	defer mgr.CloseSession(ctx, "", sessionID)

	t.Run("captures network requests from page with inline script", func(t *testing.T) {
		// Navigate to a page that makes a fetch request (will fail but still captured)
		testPage := `data:text/html,<html><body><script>
			fetch('https://example.com/api/test').catch(() => {});
		</script></body></html>`

		navReq := mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name: "navigate",
				Arguments: map[string]any{
					"sessionId": sessionID,
					"url":       testPage,
					"waitUntil": "networkidle",
				},
			},
		}
		navResult, err := navigateHandler(ctx, navReq)
		if err != nil {
			t.Fatalf("navigate failed: %v", err)
		}
		if navResult.IsError {
			t.Fatalf("navigate returned error: %v", extractTextContent(navResult.Content))
		}

		// Get network logs
		req := mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name: "get_network_logs",
				Arguments: map[string]any{
					"sessionId": sessionID,
				},
			},
		}

		result, err := networkLogsHandler(ctx, req)
		if err != nil {
			t.Fatalf("Handler returned error: %v", err)
		}

		if result.IsError {
			t.Fatalf("Expected success, got error: %s", extractTextContent(result.Content))
		}

		text := extractTextContent(result.Content)
		var logs []networkLogOutput
		if err := json.Unmarshal([]byte(text), &logs); err != nil {
			t.Fatalf("Failed to parse network logs: %v", err)
		}

		_ = logs
	})

	t.Run("verifies network log entry format", func(t *testing.T) {
		req := mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name: "get_network_logs",
				Arguments: map[string]any{
					"sessionId": sessionID,
				},
			},
		}

		result, err := networkLogsHandler(ctx, req)
		if err != nil {
			t.Fatalf("Handler returned error: %v", err)
		}

		if result.IsError {
			t.Fatalf("Expected success, got error: %s", extractTextContent(result.Content))
		}

		text := extractTextContent(result.Content)
		var logs []networkLogOutput
		if err := json.Unmarshal([]byte(text), &logs); err != nil {
			t.Fatalf("Failed to parse network logs: %v", err)
		}

		// If there are entries, verify their format
		for _, log := range logs {
			// Timestamp should be in ISO 8601 format
			if log.Timestamp == "" {
				t.Error("Expected non-empty timestamp")
			}
			// Method should be a valid HTTP method
			if log.Method == "" {
				t.Error("Expected non-empty method")
			}
			// URL should be non-empty
			if log.URL == "" {
				t.Error("Expected non-empty URL")
			}
			// DurationMs should be non-negative
			if log.DurationMs < 0 {
				t.Errorf("Expected non-negative duration, got %d", log.DurationMs)
			}
		}
	})
}

// TestTimeoutBehaviorIntegration tests timeout handling across tool operations.
func TestTimeoutBehaviorIntegration(t *testing.T) {
	pool, mgr := setupPoolAndManager(t)
	defer pool.Stop(context.Background())

	ctx := context.Background()

	// Create a session for timeout tests
	createHandler := SessionCreateHandler(mgr, DefaultTimeoutConfig())
	createReq := mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name:      "session_create",
			Arguments: map[string]any{},
		},
	}
	createResult, err := createHandler(ctx, createReq)
	if err != nil {
		t.Fatalf("Create handler returned error: %v", err)
	}
	if createResult.IsError {
		skipIfPlaywrightNotInstalled(t, extractErrorFromResult(createResult))
		t.Fatalf("Create failed: %s", extractTextContent(createResult.Content))
	}
	sessionID := strings.TrimPrefix(extractTextContent(createResult.Content), "Created session: ")
	defer mgr.CloseSession(ctx, "", sessionID)

	// Navigate to a page with content for element tests
	defaultNavHandler := NavigateHandler(mgr, DefaultTimeoutConfig(), nil)
	navReq := mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name: "navigate",
			Arguments: map[string]any{
				"sessionId": sessionID,
				"url":       "data:text/html,<html><body><h1 id='title'>Hello</h1><input id='input' type='text'/></body></html>",
			},
		},
	}
	navResult, err := defaultNavHandler(ctx, navReq)
	if err != nil {
		t.Fatalf("Navigate handler returned error: %v", err)
	}
	if navResult.IsError {
		t.Fatalf("Navigate failed: %s", extractTextContent(navResult.Content))
	}

	t.Run("element timeout with non-existent element", func(t *testing.T) {
		// Use a very short timeout to trigger timeout quickly
		shortTimeoutConfig := &TimeoutConfig{
			Default:    500 * time.Millisecond,
			Navigation: 500 * time.Millisecond,
			Element:    500 * time.Millisecond,
			Script:     500 * time.Millisecond,
		}
		clickHandler := ClickHandler(mgr, shortTimeoutConfig, nil)

		req := mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name: "click",
				Arguments: map[string]any{
					"sessionId": sessionID,
					"selector":  "#does-not-exist",
					"timeout":   float64(500),
				},
			},
		}

		start := time.Now()
		result, err := clickHandler(ctx, req)
		elapsed := time.Since(start)

		if err != nil {
			t.Fatalf("Handler returned error: %v", err)
		}

		// Should return an error (element not found or timeout)
		if !result.IsError {
			t.Fatal("Expected error for non-existent element with short timeout")
		}

		text := extractTextContent(result.Content)
		// Should contain either a timeout or element-not-found error code
		if !strings.Contains(text, "[-32002]") && !strings.Contains(text, "[-32003]") {
			t.Errorf("Expected element not found (-32002) or timeout (-32003) error, got: %s", text)
		}

		// The operation should complete within a reasonable time (not hang)
		if elapsed > 10*time.Second {
			t.Errorf("Operation took too long: %v (expected < 10s)", elapsed)
		}
	})

	t.Run("tool-level timeout parameter overrides config default", func(t *testing.T) {
		// Use a long default config, but override with short tool-level timeout
		longTimeoutConfig := &TimeoutConfig{
			Default:    60 * time.Second,
			Navigation: 60 * time.Second,
			Element:    60 * time.Second,
			Script:     60 * time.Second,
		}
		clickHandler := ClickHandler(mgr, longTimeoutConfig, nil)

		req := mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name: "click",
				Arguments: map[string]any{
					"sessionId": sessionID,
					"selector":  "#does-not-exist",
					"timeout":   float64(500), // 500ms override
				},
			},
		}

		start := time.Now()
		result, err := clickHandler(ctx, req)
		elapsed := time.Since(start)

		if err != nil {
			t.Fatalf("Handler returned error: %v", err)
		}

		if !result.IsError {
			t.Fatal("Expected error for non-existent element")
		}

		// Should complete within a few seconds despite the 60s config default,
		// because the tool-level 500ms timeout overrides it
		if elapsed > 10*time.Second {
			t.Errorf("Tool-level timeout override not working: took %v (expected < 10s)", elapsed)
		}
	})

	t.Run("context cancellation propagates through tool operation", func(t *testing.T) {
		clickHandler := ClickHandler(mgr, DefaultTimeoutConfig(), nil)

		// Create a context that will be cancelled shortly
		cancelCtx, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
		defer cancel()

		req := mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name: "click",
				Arguments: map[string]any{
					"sessionId": sessionID,
					"selector":  "#does-not-exist",
				},
			},
		}

		start := time.Now()
		result, err := clickHandler(cancelCtx, req)
		elapsed := time.Since(start)

		if err != nil {
			t.Fatalf("Handler returned error: %v", err)
		}

		// Should return an error (either timeout from context or element not found)
		if !result.IsError {
			t.Fatal("Expected error when context is cancelled")
		}

		// Should complete relatively quickly due to context cancellation
		if elapsed > 10*time.Second {
			t.Errorf("Context cancellation not propagating: took %v (expected < 10s)", elapsed)
		}
	})

	t.Run("script timeout with long-running JavaScript", func(t *testing.T) {
		// Use a very short script timeout
		shortScriptConfig := &TimeoutConfig{
			Default:    30 * time.Second,
			Navigation: 30 * time.Second,
			Element:    5 * time.Second,
			Script:     1 * time.Second,
		}
		evalHandler := EvaluateHandler(mgr, shortScriptConfig, DefaultResponseConfig())

		// JavaScript that runs for a long time (Promise that resolves after 30s)
		req := mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name: "evaluate",
				Arguments: map[string]any{
					"sessionId":  sessionID,
					"expression": "new Promise(resolve => setTimeout(resolve, 30000))",
				},
			},
		}

		start := time.Now()
		result, err := evalHandler(ctx, req)
		elapsed := time.Since(start)

		if err != nil {
			t.Fatalf("Handler returned error: %v", err)
		}

		// Should return an error (timeout)
		if !result.IsError {
			t.Fatal("Expected error for long-running script with short timeout")
		}

		// Should complete within a reasonable time (context timeout of 1s + buffer)
		if elapsed > 15*time.Second {
			t.Errorf("Script timeout not working: took %v (expected < 15s)", elapsed)
		}
	})

	t.Run("navigation timeout with unreachable host", func(t *testing.T) {
		shortNavConfig := &TimeoutConfig{
			Default:    30 * time.Second,
			Navigation: 2 * time.Second,
			Element:    5 * time.Second,
			Script:     30 * time.Second,
		}
		navHandler := NavigateHandler(mgr, shortNavConfig, nil)

		// Use a non-routable IP address that will timeout
		req := mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name: "navigate",
				Arguments: map[string]any{
					"sessionId": sessionID,
					"url":       "http://192.0.2.1/timeout-test",
					"timeout":   float64(2000),
				},
			},
		}

		start := time.Now()
		result, err := navHandler(ctx, req)
		elapsed := time.Since(start)

		if err != nil {
			t.Fatalf("Handler returned error: %v", err)
		}

		// Should return an error
		if !result.IsError {
			t.Fatal("Expected error for unreachable URL with short timeout")
		}

		text := extractTextContent(result.Content)
		// Should contain a timeout or navigation error
		if !strings.Contains(text, "[-32003]") && !strings.Contains(text, "[-32004]") {
			t.Errorf("Expected timeout (-32003) or navigation failed (-32004) error, got: %s", text)
		}

		// Should not hang - should complete within reasonable time
		if elapsed > 30*time.Second {
			t.Errorf("Navigation timeout not working: took %v (expected < 30s)", elapsed)
		}
	})
}

// TestAccessibilityTreeSemanticRolesIntegration exercises accessibilityTreeJS
// with diverse HTML structures to verify semantic role mapping, ARIA attribute
// extraction, heading levels, form controls, and hidden element exclusion.
// snapshotAttrsRe matches the bracketed attributes after a snapshot line's name,
// ending with its ref: e<n> on the first document, f<k>e<n> after.
const snapshotAttrsRe = `(?: \[[^\]]+\])* \[ref=(?:f\d+)?e\d+\](?: \[[^\]]+\])*`

func TestAccessibilityTreeSemanticRolesIntegration(t *testing.T) {
	pool, mgr := setupPoolAndManager(t)
	defer pool.Stop(context.Background())

	createHandler := SessionCreateHandler(mgr, DefaultTimeoutConfig())
	navigateHandler := NavigateHandler(mgr, DefaultTimeoutConfig(), nil)
	accessibilityHandler := GetAccessibilityTreeHandler(mgr, DefaultTimeoutConfig(), DefaultResponseConfig())
	ctx := context.Background()
	ts := startTestHTTPServer(t)

	// Create a session
	createReq := mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name:      "session_create",
			Arguments: map[string]any{},
		},
	}
	createResult, err := createHandler(ctx, createReq)
	if err != nil {
		t.Fatalf("Create handler returned error: %v", err)
	}
	if createResult.IsError {
		skipIfPlaywrightNotInstalled(t, extractErrorFromResult(createResult))
		t.Fatalf("Create failed: %s", extractTextContent(createResult.Content))
	}
	sessionID := strings.TrimPrefix(extractTextContent(createResult.Content), "Created session: ")
	defer mgr.CloseSession(ctx, "", sessionID)

	navigate := func(t *testing.T, url string) {
		t.Helper()
		navReq := mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name: "navigate",
				Arguments: map[string]any{
					"sessionId": sessionID,
					"url":       url,
				},
			},
		}
		if result, err := navigateHandler(ctx, navReq); err != nil || result.IsError {
			t.Fatalf("Navigation failed: %v %s", err, extractTextContent(result.Content))
		}
	}

	getTree := func(t *testing.T) string {
		t.Helper()
		req := mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name: "get_accessibility_tree",
				Arguments: map[string]any{
					"sessionId": sessionID,
				},
			},
		}
		result, err := accessibilityHandler(ctx, req)
		if err != nil {
			t.Fatalf("Handler returned error: %v", err)
		}
		if result.IsError {
			t.Fatalf("Expected success, got error: %s", extractTextContent(result.Content))
		}
		return extractTextContent(result.Content)
	}

	// navigateAndGetTree navigates to a URL and returns the YAML snapshot.
	navigateAndGetTree := func(t *testing.T, url string) string {
		t.Helper()
		navigate(t, url)
		return getTree(t)
	}

	activePage := func(t *testing.T) playwright.Page {
		t.Helper()
		sess, ok := mgr.GetSession("", sessionID)
		if !ok || sess.ActivePage() == nil {
			t.Fatal("Expected an active page")
		}
		return sess.ActivePage()
	}

	t.Run("semantic HTML roles", func(t *testing.T) {
		text := navigateAndGetTree(t, ts.URL+"/a11y-semantic")

		for _, role := range []string{"navigation", "main", "heading", "link", "article", "complementary", "contentinfo"} {
			if !strings.Contains(text, "- "+role) {
				t.Errorf("Expected role '%s' in snapshot:\n%s", role, text)
			}
		}
		for _, want := range []string{`navigation "Main Navigation"`, `link "Home"`, `heading "Page Title"`} {
			if !strings.Contains(text, want) {
				t.Errorf("Expected %s in snapshot:\n%s", want, text)
			}
		}
	})

	t.Run("form controls show values and states", func(t *testing.T) {
		navigate(t, ts.URL+"/a11y-forms")
		page := activePage(t)
		if err := page.Locator("#name").Fill("Ada Lovelace"); err != nil {
			t.Fatalf("fill failed: %v", err)
		}
		if err := page.Locator("#agree").Check(); err != nil {
			t.Fatalf("check failed: %v", err)
		}
		text := getTree(t)

		for _, role := range []string{"textbox", "checkbox", "combobox", "button"} {
			if !strings.Contains(text, "- "+role) {
				t.Errorf("Expected role '%s' in snapshot:\n%s", role, text)
			}
		}
		if !regexp.MustCompile(`textbox "Name"` + snapshotAttrsRe + `: Ada Lovelace`).MatchString(text) {
			t.Errorf("Expected the filled textbox to show its value:\n%s", text)
		}
		if !regexp.MustCompile(`checkbox "I agree to terms" \[checked\]`).MatchString(text) {
			t.Errorf("Expected the checked checkbox to show [checked]:\n%s", text)
		}
		if !strings.Contains(text, `combobox "Select role"`) {
			t.Errorf("Expected aria-label 'Select role':\n%s", text)
		}
		if !regexp.MustCompile(`button "Submit" \[expanded=false\]|button "Submit"`).MatchString(text) {
			t.Errorf("Expected the submit button:\n%s", text)
		}
	})

	t.Run("password redaction", func(t *testing.T) {
		navigate(t, ts.URL+"/a11y-password")
		page := activePage(t)
		const secret = "hunter2: correct horse"
		if err := page.Locator("#user").Fill("ada"); err != nil {
			t.Fatalf("fill failed: %v", err)
		}
		if err := page.Locator("#pass").Fill(secret); err != nil {
			t.Fatalf("fill failed: %v", err)
		}
		// A placeholder that differs from the name puts the value on a child line;
		// a name containing ": " puts the key in single quotes.
		if err := page.Locator("#pass-placeholder").Fill("placeholder-secret"); err != nil {
			t.Fatalf("fill failed: %v", err)
		}
		if err := page.Locator("#pass-quoted").Fill("quoted: secret"); err != nil {
			t.Fatalf("fill failed: %v", err)
		}
		// AI mode gives these no ref (not interactable) but still prints their values.
		if _, err := page.Evaluate(`() => {
			document.querySelector('#pass-covered').value = 'covered-secret';
			document.querySelector('#pass-tiny').value = 'tiny-secret';
			document.querySelector('#pass-searchbox').value = 'searchbox-secret';
			document.querySelector('#pass-combobox').value = 'combobox-secret';
			document.querySelector('#pass-regex').value = 'regex-secret';
		}`); err != nil {
			t.Fatalf("setting values failed: %v", err)
		}
		text := getTree(t)

		if !regexp.MustCompile(`textbox "Password"` + snapshotAttrsRe + `: •••`).MatchString(text) {
			t.Errorf("Expected the password value as •••:\n%s", text)
		}
		if !strings.Contains(text, "/placeholder: Enter your PIN") || !strings.Contains(text, "- text: •••") {
			t.Errorf("Expected the block-form password value as •••:\n%s", text)
		}
		for _, part := range []string{secret, "hunter2", "correct horse", "placeholder-secret", "quoted: secret", "covered-secret", "tiny-secret", "searchbox-secret", "combobox-secret", "regex-secret", "frame-secret"} {
			if strings.Contains(text, part) {
				t.Errorf("Password value %q leaked into snapshot:\n%s", part, text)
			}
		}
		if !strings.Contains(text, `textbox "Username"`) || !strings.Contains(text, ": ada") {
			t.Errorf("Expected the username value to stay visible:\n%s", text)
		}
	})

	t.Run("hidden elements excluded from tree nodes", func(t *testing.T) {
		text := navigateAndGetTree(t, ts.URL+"/a11y-hidden")

		if !strings.Contains(text, `button "Visible Button"`) {
			t.Errorf("Expected 'Visible Button' in snapshot:\n%s", text)
		}
		// display:none and visibility:hidden content is left out.
		for _, hidden := range []string{"Hidden text", "Invisible text", "Hidden Button", "Invisible Button"} {
			if strings.Contains(text, hidden) {
				t.Errorf("Expected %q to be absent:\n%s", hidden, text)
			}
		}
		// AI mode keeps on-screen content even when it is aria-hidden, since an
		// agent may need to act on it (decision D1 on issue #123).
		if !strings.Contains(text, "Decorative Button") {
			t.Errorf("Expected aria-hidden but visible 'Decorative Button' to be kept:\n%s", text)
		}
	})

	t.Run("heading levels", func(t *testing.T) {
		text := navigateAndGetTree(t, ts.URL+"/a11y-headings")

		for _, level := range []int{1, 2, 3, 4} {
			want := fmt.Sprintf(`heading "Level %d" [level=%d]`, level, level)
			if !strings.Contains(text, want) {
				t.Errorf("Expected %s in snapshot:\n%s", want, text)
			}
		}
	})

	t.Run("lists and list items", func(t *testing.T) {
		text := navigateAndGetTree(t, ts.URL+"/a11y-lists")

		for _, want := range []string{"- list", "- listitem", "Apple", "First"} {
			if !strings.Contains(text, want) {
				t.Errorf("Expected %q in snapshot:\n%s", want, text)
			}
		}
	})

	t.Run("aria-labelledby resolves to referenced element text", func(t *testing.T) {
		text := navigateAndGetTree(t, ts.URL+"/a11y-labelledby")

		if !strings.Contains(text, `region "Settings"`) {
			t.Errorf("Expected region named 'Settings' from aria-labelledby:\n%s", text)
		}
	})

	t.Run("tree has children structure", func(t *testing.T) {
		text := navigateAndGetTree(t, ts.URL+"/a11y-semantic")

		// Children are indented one level under their parent.
		m := regexp.MustCompile(`(?m)^( *)- main` + snapshotAttrsRe + `:\n( *)- heading "Page Title"`).FindStringSubmatch(text)
		if m == nil || len(m[2]) != len(m[1])+2 {
			t.Errorf("Expected the heading nested one level under main:\n%s", text)
		}
	})

	t.Run("input type mapping", func(t *testing.T) {
		text := navigateAndGetTree(t, ts.URL+"/a11y-forms")

		// text input, email input, and textarea all map to textbox
		if count := strings.Count(text, "- textbox"); count < 3 {
			t.Errorf("Expected at least 3 textbox roles (text, email, textarea), found %d:\n%s", count, text)
		}
	})
}

// TestNavigateAndExtractTextMarkdownIntegration exercises extractTextAsMarkdownJS
// to verify that navigate_and_extract_text returns clean markdown output for
// headings, links, lists, formatting, tables, code blocks, blockquotes, and
// that it excludes hidden elements and script/style content.
func TestNavigateAndExtractTextMarkdownIntegration(t *testing.T) {
	pool, mgr := setupPoolAndManager(t)
	defer pool.Stop(context.Background())

	handler := NavigateAndExtractTextHandler(mgr, DefaultTimeoutConfig(), DefaultResponseConfig())
	ctx := context.Background()
	ts := startTestHTTPServer(t)

	// extractMarkdown is a helper that calls navigate_and_extract_text and
	// returns the resulting markdown text.
	extractMarkdown := func(t *testing.T, args map[string]any) string {
		t.Helper()
		req := mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name:      "navigate_and_extract_text",
				Arguments: args,
			},
		}
		result, err := handler(ctx, req)
		if err != nil {
			t.Fatalf("Handler returned error: %v", err)
		}
		if result.IsError {
			skipIfPlaywrightNotInstalled(t, extractErrorFromResult(result))
			t.Fatalf("Expected success, got error: %s", extractTextContent(result.Content))
		}
		return extractTextContent(result.Content)
	}

	t.Run("headings render as markdown", func(t *testing.T) {
		text := extractMarkdown(t, map[string]any{"url": ts.URL + "/md-headings"})

		if !strings.Contains(text, "# Main Title") {
			t.Errorf("Expected '# Main Title' in output, got:\n%s", text)
		}
		if !strings.Contains(text, "## Subtitle") {
			t.Errorf("Expected '## Subtitle' in output, got:\n%s", text)
		}
		if !strings.Contains(text, "### Section") {
			t.Errorf("Expected '### Section' in output, got:\n%s", text)
		}
		if !strings.Contains(text, "Body text") {
			t.Errorf("Expected 'Body text' in output, got:\n%s", text)
		}
		// Should NOT contain JSON structure
		if strings.Contains(text, `"tag"`) || strings.Contains(text, `"children"`) {
			t.Error("Output should be markdown, not JSON")
		}
	})

	t.Run("links render as markdown", func(t *testing.T) {
		text := extractMarkdown(t, map[string]any{"url": ts.URL + "/md-links"})

		if !strings.Contains(text, "[Example Site](https://example.com)") {
			t.Errorf("Expected markdown link in output, got:\n%s", text)
		}
	})

	t.Run("relative links", func(t *testing.T) {
		text := extractMarkdown(t, map[string]any{"url": ts.URL + "/md-structure"})

		if want := "[About](" + ts.URL + "/about)"; !strings.Contains(text, want) {
			t.Errorf("Expected %q in output, got:\n%s", want, text)
		}
		if !strings.Contains(text, "Script link") || strings.Contains(text, "javascript:") {
			t.Errorf("Expected javascript: link rendered as text only, got:\n%s", text)
		}
	})

	t.Run("images", func(t *testing.T) {
		text := extractMarkdown(t, map[string]any{"url": ts.URL + "/md-structure"})

		if want := "![Logo](" + ts.URL + "/a.png)"; !strings.Contains(text, want) {
			t.Errorf("Expected %q in output, got:\n%s", want, text)
		}
	})

	t.Run("heading links", func(t *testing.T) {
		text := extractMarkdown(t, map[string]any{"url": ts.URL + "/md-structure"})

		if want := "## [Docs](" + ts.URL + "/x)"; !strings.Contains(text, want) {
			t.Errorf("Expected %q in output, got:\n%s", want, text)
		}
	})

	t.Run("nested table", func(t *testing.T) {
		text := extractMarkdown(t, map[string]any{"url": ts.URL + "/md-structure"})

		if n := strings.Count(text, "Outer row"); n != 1 {
			t.Errorf("Expected outer row once, found %d times:\n%s", n, text)
		}
		if n := strings.Count(text, "Inner A"); n != 1 {
			t.Errorf("Expected nested cell text once, found %d times:\n%s", n, text)
		}
		if !strings.Contains(text, "| Outer row | Inner A Inner B |") {
			t.Errorf("Expected nested table flattened into its cell, got:\n%s", text)
		}
		if want := "| Linked | [Cell link](" + ts.URL + "/cell) |"; !strings.Contains(text, want) {
			t.Errorf("Expected %q in output, got:\n%s", want, text)
		}
	})

	t.Run("form controls", func(t *testing.T) {
		text := extractMarkdown(t, map[string]any{"url": ts.URL + "/md-forms"})

		for _, want := range []string{
			`[textbox "Email": a@b.c]`,
			`[checkbox "Subscribe": checked]`,
			`[checkbox "Remember me": unchecked]`,
			`[combobox "Role": User]`,
			`[textbox "Search box": query]`,
			`[spinbutton "Quantity": 3]`,
			`[textbox "Biography": Hello there]`,
			`[radio "color": unchecked]`,
			`[button "Send"]`,
			`[button "Go"]`,
		} {
			if !strings.Contains(text, want) {
				t.Errorf("Expected %s in output, got:\n%s", want, text)
			}
		}
		if strings.Contains(text, "hidden-token-value") || strings.Contains(text, "csrf") {
			t.Errorf("type=hidden input should be skipped, got:\n%s", text)
		}
	})

	t.Run("password redaction", func(t *testing.T) {
		text := extractMarkdown(t, map[string]any{"url": ts.URL + "/md-forms"})

		if !strings.Contains(text, `[password "Password": set]`) {
			t.Errorf("Expected filled password token, got:\n%s", text)
		}
		if !strings.Contains(text, `[password "Confirm": empty]`) {
			t.Errorf("Expected empty password token, got:\n%s", text)
		}
		if strings.Contains(text, "hunter2") {
			t.Errorf("Password value must never appear in output, got:\n%s", text)
		}
	})

	t.Run("secret field redaction", func(t *testing.T) {
		text := extractMarkdown(t, map[string]any{"url": ts.URL + "/md-forms"})

		for _, want := range []string{
			`[password "Current": set]`,
			`[password "Login code": set]`,
			`[password "PIN": set]`,
			`[password "New": set]`,
			`[password "Card": set]`,
			`[password "CVC": set]`,
			`[password "Billing card": set]`,
			`[password "Code": set]`,
			`[password "Recovery phrase": set]`,
			`[password "Code area": set]`,
			`[textbox "Contact": me@example.org]`,
			`[checkbox "Masked box": checked]`,
			`[button "Masked send"]`,
			`[textbox "Echo"`,
			`[password "Custom PIN": set]`,
			`[password "Wrapped PIN": set]`,
			`[checkbox "Box": unchecked]`,
			`[textbox "Save draft"`,
		} {
			if !strings.Contains(text, want) {
				t.Errorf("Expected %s in output, got:\n%s", want, text)
			}
		}
		for _, secret := range []string{
			"shown-secret", "981234", "masked-pin", "new-secret", "4111111111111111", "737",
			"5555444433331111", "246810", "secret-phrase", "135790",
			"pre-secret", "code-secret", "typed-pin", "wrap-secret", "labelled-secret", "box-secret",
			"pre-ed-secret",
		} {
			if strings.Contains(text, secret) {
				t.Errorf("Secret value %q must never appear in output, got:\n%s", secret, text)
			}
		}
	})

	t.Run("code blocks in lists and blockquotes", func(t *testing.T) {
		text := extractMarkdown(t, map[string]any{"url": ts.URL + "/md-nested-code"})

		for _, want := range []string{
			"1. ```\n     indented\n       more\n   ```",
			// Code after prose in an item keeps the item's indent on every line.
			"1. step\n\n   ```\n     sx\n   ```",
			"- outer\n\n  - ```\n      ny\n    ```",
			"> ```\n>   quoted\n> ```",
			// In a list inside a blockquote, the fence keeps the item's indent after "> ".
			"> - s\n>\n>   ```\n>     qx\n>   ```",
			"a b",
			"```\n  x\n```",
			"c d",
			// A line inside the code that starts with a fence does not end the block.
			"```\n```\ncode\n```\n  after\n```",
			"e f",
			// Sentinel characters from every page source are stripped before the tidy
			// pass, so none of the groups pairs into a code segment.
			"g1\n\nh1", "g2\n\nh2", "g3\n\nh3", "g4\n\nh4", "g5\n\nh5", "g6\n\nh6",
			// The content next to a stripped sentinel is kept.
			"`c`", `[textbox "First": v]`, "![p](" + ts.URL + "/i.png)",
			"```\n  k  m\n```",
		} {
			if !strings.Contains(text, want) {
				t.Errorf("Expected %q in output, got:\n%s", want, text)
			}
		}
		if strings.Contains(text, "a   b") || strings.Contains(text, "c   d") || strings.Contains(text, "e   f") {
			t.Errorf("Expected prose after code blocks to be tidied, got:\n%s", text)
		}
		if strings.ContainsAny(text, "\x00\x01") {
			t.Errorf("Code block sentinels must be stripped, got:\n%q", text)
		}
	})

	t.Run("shadow dom", func(t *testing.T) {
		text := extractMarkdown(t, map[string]any{"url": ts.URL + "/md-shadow"})

		for _, want := range []string{"Shadow text before", "Shadow text after", "Second shadow root"} {
			if !strings.Contains(text, want) {
				t.Errorf("Expected shadow root text %q, got:\n%s", want, text)
			}
		}
		if n := strings.Count(text, "Slotted light text"); n != 1 {
			t.Errorf("Expected slotted light-DOM text exactly once, found %d times:\n%s", n, text)
		}
		if before, after := strings.Index(text, "Shadow text before"), strings.Index(text, "Slotted light text"); before > after {
			t.Errorf("Expected slotted text rendered in place of its slot, got:\n%s", text)
		}
		if strings.Contains(text, "Unslotted light text") {
			t.Errorf("Light-DOM children with no slot are not rendered by the browser, got:\n%s", text)
		}
	})

	t.Run("iframe placeholder", func(t *testing.T) {
		text := extractMarkdown(t, map[string]any{"url": ts.URL + "/md-iframe"})

		if n := strings.Count(text, `[iframe "Payment form"]`); n != 1 {
			t.Errorf("Expected one titled iframe token, found %d:\n%s", n, text)
		}
		if want := `[iframe "` + ts.URL + `/md-headings"]`; strings.Count(text, want) != 1 {
			t.Errorf("Expected one %s token, got:\n%s", want, text)
		}
		if n := strings.Count(text, "[iframe "); n != 2 {
			t.Errorf("Expected 2 iframe tokens, found %d:\n%s", n, text)
		}
		if strings.Contains(text, "Inside titled frame") || strings.Contains(text, "Main Title") {
			t.Errorf("Iframe content should be absent, got:\n%s", text)
		}
		if !strings.Contains(text, "Before frames") || !strings.Contains(text, "After frames") {
			t.Errorf("Expected surrounding text, got:\n%s", text)
		}
	})

	t.Run("unordered and ordered lists", func(t *testing.T) {
		text := extractMarkdown(t, map[string]any{"url": ts.URL + "/md-lists"})

		// Unordered list
		if !strings.Contains(text, "- Apple") {
			t.Errorf("Expected '- Apple' in output, got:\n%s", text)
		}
		if !strings.Contains(text, "- Banana") {
			t.Errorf("Expected '- Banana' in output, got:\n%s", text)
		}
		if !strings.Contains(text, "- Cherry") {
			t.Errorf("Expected '- Cherry' in output, got:\n%s", text)
		}
		// Ordered list
		if !strings.Contains(text, "1. First") {
			t.Errorf("Expected '1. First' in output, got:\n%s", text)
		}
		if !strings.Contains(text, "2. Second") {
			t.Errorf("Expected '2. Second' in output, got:\n%s", text)
		}
		if !strings.Contains(text, "3. Third") {
			t.Errorf("Expected '3. Third' in output, got:\n%s", text)
		}
	})

	t.Run("bold and italic formatting", func(t *testing.T) {
		text := extractMarkdown(t, map[string]any{"url": ts.URL + "/md-formatting"})

		if !strings.Contains(text, "**bold text**") {
			t.Errorf("Expected **bold text** in output, got:\n%s", text)
		}
		if !strings.Contains(text, "*italic text*") {
			t.Errorf("Expected *italic text* in output, got:\n%s", text)
		}
	})

	t.Run("tables render as pipe-delimited markdown", func(t *testing.T) {
		text := extractMarkdown(t, map[string]any{"url": ts.URL + "/md-table"})

		if !strings.Contains(text, "| Name | Age |") {
			t.Errorf("Expected table header row in output, got:\n%s", text)
		}
		if !strings.Contains(text, "| --- | --- |") {
			t.Errorf("Expected table separator in output, got:\n%s", text)
		}
		if !strings.Contains(text, "| Alice | 30 |") {
			t.Errorf("Expected table data row in output, got:\n%s", text)
		}
		if !strings.Contains(text, "| Bob | 25 |") {
			t.Errorf("Expected table data row in output, got:\n%s", text)
		}
	})

	t.Run("hidden elements and script/style excluded", func(t *testing.T) {
		text := extractMarkdown(t, map[string]any{"url": ts.URL + "/md-hidden"})

		if !strings.Contains(text, "Visible paragraph") {
			t.Errorf("Expected 'Visible paragraph', got:\n%s", text)
		}
		if !strings.Contains(text, "Another visible paragraph") {
			t.Errorf("Expected 'Another visible paragraph', got:\n%s", text)
		}
		if strings.Contains(text, "Hidden paragraph") {
			t.Error("Hidden paragraph (display:none) should not appear in output")
		}
		if strings.Contains(text, "Invisible paragraph") {
			t.Error("Invisible paragraph (visibility:hidden) should not appear in output")
		}
		if strings.Contains(text, "script content") {
			t.Error("Script content should not appear in output")
		}
		if strings.Contains(text, "Aria hidden paragraph") || strings.Contains(text, "decorative icon") {
			t.Errorf("aria-hidden text should not appear in output, got:\n%s", text)
		}
		if !strings.Contains(text, "Shown label") {
			t.Errorf("Expected visible siblings of aria-hidden text, got:\n%s", text)
		}
		if strings.Contains(text, "color: red") {
			t.Error("Style content should not appear in output")
		}
	})

	t.Run("inline and fenced code blocks", func(t *testing.T) {
		text := extractMarkdown(t, map[string]any{"url": ts.URL + "/md-code"})

		// Inline code should be wrapped in backticks
		if !strings.Contains(text, "`fmt.Println`") {
			t.Errorf("Expected inline code with backticks, got:\n%s", text)
		}
		// Pre/code block content should be present
		if !strings.Contains(text, "func main()") {
			t.Errorf("Expected code block content, got:\n%s", text)
		}
		if !strings.Contains(text, "```") {
			t.Errorf("Expected fenced code block markers, got:\n%s", text)
		}
	})

	t.Run("blockquotes", func(t *testing.T) {
		text := extractMarkdown(t, map[string]any{"url": ts.URL + "/md-blockquote"})

		if !strings.Contains(text, "> ") {
			t.Errorf("Expected blockquote with '> ' prefix, got:\n%s", text)
		}
		if !strings.Contains(text, "famous author") {
			t.Errorf("Expected blockquote content, got:\n%s", text)
		}
	})

	t.Run("selector limits extraction scope", func(t *testing.T) {
		text := extractMarkdown(t, map[string]any{
			"url":      ts.URL + "/md-scoped",
			"selector": "#content",
		})

		if !strings.Contains(text, "Main Content") {
			t.Errorf("Expected 'Main Content' in scoped output, got:\n%s", text)
		}
		if !strings.Contains(text, "Important text") {
			t.Errorf("Expected 'Important text' in scoped output, got:\n%s", text)
		}
		// Content outside selector should not appear
		if strings.Contains(text, "Page Header") {
			t.Error("Content outside selector (#content) should not appear")
		}
		if strings.Contains(text, "Footer text") {
			t.Error("Content outside selector (#content) should not appear")
		}
	})
}
