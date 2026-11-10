package web

import (
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"testing"
)

// The strict TLS configuration must not lock out a key exchange: neither the post-quantum hybrid nor the classic ones.
func TestStrictTLSConfig_negotiatesKeyExchanges(t *testing.T) {
	srv := httptest.NewUnstartedServer(http.NotFoundHandler())
	srv.TLS = strictTLSConfig()
	srv.StartTLS()
	defer srv.Close()

	for name, curve := range map[string]tls.CurveID{
		"X25519MLKEM768": tls.X25519MLKEM768,
		"X25519":         tls.X25519,
		"P-256":          tls.CurveP256,
	} {
		t.Run(name, func(t *testing.T) {
			client := srv.Client()
			transport := client.Transport.(*http.Transport).Clone()
			transport.TLSClientConfig.CurvePreferences = []tls.CurveID{curve}

			resp, err := (&http.Client{Transport: transport}).Get(srv.URL)
			if err != nil {
				t.Fatalf("handshake failed: %v", err)
			}

			defer func() { _ = resp.Body.Close() }()

			if resp.TLS.CurveID != curve {
				t.Errorf("negotiated %v, want %v", resp.TLS.CurveID, curve)
			}
		})
	}
}
