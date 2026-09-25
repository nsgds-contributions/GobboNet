package setup

import (
	"net/http"
	"os"
	"testing"

	"github.com/ElodineOfficial/GobboNet/internal/config"
	"github.com/ElodineOfficial/GobboNet/internal/keyring"
)

// The keyring is the password check on an encrypted install. Writing a hash over
// its marker would leave a password that does not log in beside one that does.
func TestEncryptedInstallKeepsItsPassword(t *testing.T) {
	s := newTestServer(t)
	if err := os.MkdirAll(s.cfg.DataDir, 0o700); err != nil {
		t.Fatal(err)
	}
	const marker = "$gobbonet-keyring$"
	if err := config.Set(s.cfg.Path, "access_secret", marker); err != nil {
		t.Fatal(err)
	}
	s.cfg.AccessSecret = marker
	if _, _, err := keyring.Create(s.cfg.KeyringPath(), "original-password"); err != nil {
		t.Fatal(err)
	}
	if !passwordConfigured(*s.cfg) {
		t.Fatal("an encrypted install reports no password")
	}
	s.passwordSet = passwordConfigured(*s.cfg)

	rec := post(t, s, "/api/password", `{"password":"brand-new-password","confirm":"brand-new-password"}`)
	if rec.Code != http.StatusConflict {
		t.Fatalf("got %d, want 409: %s", rec.Code, rec.Body)
	}
	got, err := config.Load(s.cfg.Path)
	if err != nil {
		t.Fatal(err)
	}
	if got.AccessSecret != marker {
		t.Fatal("the keyring marker was overwritten")
	}
	if _, err := keyring.UnlockPassword(s.cfg.KeyringPath(), "original-password"); err != nil {
		t.Fatalf("the original password no longer unlocks: %v", err)
	}
}
