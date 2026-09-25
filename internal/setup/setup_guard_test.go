package setup

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ElodineOfficial/GobboNet/internal/auth"
	"github.com/ElodineOfficial/GobboNet/internal/config"
)

// A page on another site can post a text/plain form to the wizard without any
// CORS preflight, and a DNS-rebound page can read it. Each of those shapes must
// be refused before a handler runs; the wizard's own page still works.
func TestWizardRefusesRequestsFromOtherPages(t *testing.T) {
	body := `{"password":"attacker-password","confirm":"attacker-password"}`
	cases := []struct {
		name  string
		host  string
		hdr   map[string]string
		wants int
	}{
		{"cross-site text/plain form", testHost, map[string]string{"Content-Type": "text/plain", "Origin": "http://evil.example"}, http.StatusForbidden},
		{"text/plain from its own origin", testHost, map[string]string{"Content-Type": "text/plain"}, http.StatusUnsupportedMediaType},
		{"JSON from another origin", testHost, map[string]string{"Content-Type": "application/json", "Origin": "http://evil.example"}, http.StatusForbidden},
		{"browser says cross-site", testHost, map[string]string{"Content-Type": "application/json", "Sec-Fetch-Site": "cross-site"}, http.StatusForbidden},
		{"DNS-rebound Host", "evil.example:9", map[string]string{"Content-Type": "application/json"}, http.StatusForbidden},
		{"the wizard's own page", testHost, map[string]string{"Content-Type": "application/json", "Origin": "http://" + testHost, "Sec-Fetch-Site": "same-origin"}, http.StatusOK},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s := newTestServer(t)
			req := httptest.NewRequest(http.MethodPost, "/api/password", strings.NewReader(body))
			req.Host = c.host
			for k, v := range c.hdr {
				req.Header.Set(k, v)
			}
			rec := httptest.NewRecorder()
			s.routes().ServeHTTP(rec, req)
			if rec.Code != c.wants {
				t.Fatalf("got %d, want %d: %s", rec.Code, c.wants, rec.Body)
			}
		})
	}

	s := newTestServer(t)
	page := httptest.NewRequest(http.MethodGet, "/", nil)
	page.Host = testHost
	pageRec := httptest.NewRecorder()
	s.routes().ServeHTTP(pageRec, page)
	if pageRec.Code != http.StatusOK || pageRec.Header().Get("X-Frame-Options") != "DENY" ||
		!strings.Contains(pageRec.Header().Get("Content-Security-Policy"), "frame-ancestors 'none'") {
		t.Errorf("the wizard page can be framed: %d %q %q", pageRec.Code,
			pageRec.Header().Get("X-Frame-Options"), pageRec.Header().Get("Content-Security-Policy"))
	}

	req := httptest.NewRequest(http.MethodGet, "/api/state", nil)
	req.Host = "evil.example:9"
	rec := httptest.NewRecorder()
	s.routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Errorf("a DNS-rebound read of /api/state got %d, want 403", rec.Code)
	}
}

// When serve opens the wizard on an install that already has a password -- to
// finish an abandoned setup, or to offer a chat model after the models went --
// the wizard must not become a way to replace that password.
func TestServesWizardKeepsAnExistingPassword(t *testing.T) {
	s := newTestServer(t)
	secret, err := auth.NewSecret("the-original-password")
	if err != nil {
		t.Fatal(err)
	}
	if err := config.Set(s.cfg.Path, "access_secret", secret); err != nil {
		t.Fatal(err)
	}
	s.cfg.AccessSecret = secret
	s.passwordSet = true
	s.opts.KeepPassword = true

	rec := post(t, s, "/api/password", `{"password":"brand-new-password","confirm":"brand-new-password"}`)
	if rec.Code != http.StatusConflict {
		t.Fatalf("got %d, want 409: %s", rec.Code, rec.Body)
	}
	got, err := config.Load(s.cfg.Path)
	if err != nil {
		t.Fatal(err)
	}
	if got.AccessSecret != secret {
		t.Fatal("the existing password was replaced")
	}

	// An explicit `gobbonet setup --force` still may.
	s.opts.KeepPassword = false
	if rec := post(t, s, "/api/password", `{"password":"brand-new-password","confirm":"brand-new-password"}`); rec.Code != http.StatusOK {
		t.Fatalf("explicit setup: got %d, want 200: %s", rec.Code, rec.Body)
	}
}
