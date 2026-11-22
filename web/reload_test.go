package web

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/dreitier/backmon/config"
)

// reloadConfiguration rewrites ./config.yaml of TestMain with the help of edit, reloads it and restores the
// original configuration when the test is done
func reloadConfiguration(t *testing.T, edit func(string) string) {
	t.Helper()

	original, err := os.ReadFile(config.CfgFileName)
	if err != nil {
		t.Fatal(err)
	}

	write := func(content string) {
		if err := os.WriteFile(config.CfgFileName, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}

		if _, _, err := config.Reload(); err != nil {
			t.Fatal(err)
		}
	}

	t.Cleanup(func() { write(string(original)) })
	write(edit(string(original)))
}

func TestRouter_reloadedBasicAuthCredentialsApplyWithoutRestart(t *testing.T) {
	reloadConfiguration(t, func(cfg string) string {
		return strings.Replace(cfg, "password: "+testPassword, "password: rotated", 1)
	})

	if rec := request(http.MethodGet, "/api", true); rec.Code != http.StatusUnauthorized {
		t.Errorf("GET /api with the previous password: status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}

	req := httptest.NewRequest(http.MethodGet, "/api", nil)
	req.SetBasicAuth(testUsername, "rotated")
	rec := httptest.NewRecorder()
	GetInstance().endpointsRouter.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("GET /api with the new password: status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestRouter_reloadedConfigurationCanRemoveBasicAuth(t *testing.T) {
	reloadConfiguration(t, func(cfg string) string {
		return strings.Replace(cfg, "http:", "ignored:", 1)
	})

	if rec := request(http.MethodGet, "/api", false); rec.Code != http.StatusOK {
		t.Errorf("GET /api without credentials: status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestRouter_reloadedConfigurationCanDisableDownloads(t *testing.T) {
	groups := getResource(t, apiPath(testDiskName, "my-backups", "pgdump")).names("variants")
	if len(groups) != 1 {
		t.Fatalf("groups = %v, want exactly one", groups)
	}

	download := apiPath(testDiskName, "my-backups", "pgdump", groups[0])

	if rec := request(http.MethodGet, download, true); rec.Code != http.StatusOK {
		t.Fatalf("download while enabled: status = %d, want %d", rec.Code, http.StatusOK)
	}

	reloadConfiguration(t, func(cfg string) string {
		return strings.Replace(cfg, "enabled: true", "enabled: false", 1)
	})

	if rec := request(http.MethodGet, download, true); rec.Code != http.StatusNotFound {
		t.Errorf("download while disabled: status = %d, want %d", rec.Code, http.StatusNotFound)
	}

	// the other routes of the API are not affected
	if rec := request(http.MethodGet, apiPath(testDiskName, "my-backups", "pgdump"), true); rec.Code != http.StatusOK {
		t.Errorf("file resource while downloads are disabled: status = %d, want %d", rec.Code, http.StatusOK)
	}
}
