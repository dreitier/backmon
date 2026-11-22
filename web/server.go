package web

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	log "github.com/sirupsen/logrus"

	"github.com/dreitier/backmon/config"
)

// StartServer blocks while the webserver is running and returns the error which stopped it
func StartServer() error {
	port := config.GetInstance().Global().HttpPort()
	listenAddr := fmt.Sprintf(":%d", port)

	log.Infof("Starting webserver on %s", listenAddr)

	// #11: support for TLS configuration
	// optional tls.Config
	var tlsServerConfig *tls.Config = nil
	// by default, we are not configuring TLS ciphers and let them as it is
	var tlsNextProto map[string]func(*http.Server, *tls.Conn, http.Handler) = nil
	// get user-defined TLS configuration from config.yaml
	userDefinedTlsConfiguration := config.GetInstance().Http().Tls

	if userDefinedTlsConfiguration != nil {
		// `strict: true` sets the TLS configuration to something SSLLabs prefers
		// @see https://gist.github.com/denji/12b3a568f092ab951456
		if userDefinedTlsConfiguration.IsStrict {
			tlsServerConfig = strictTLSConfig()
		}

		// provide an empty hashmap to disable any other TLS ciphers
		restrictedProtos := make(map[string]func(*http.Server, *tls.Conn, http.Handler))
		tlsNextProto = restrictedProtos
	}

	srv := &http.Server{
		Handler:      newHandler(),
		Addr:         listenAddr,
		TLSConfig:    tlsServerConfig,
		TLSNextProto: tlsNextProto,
		// protect against slowloris attacks; no WriteTimeout because large backup downloads are streamed
		ReadHeaderTimeout: 10 * time.Second,
	}

	// if the user has provided a TLS configuration, start with TLS
	if userDefinedTlsConfiguration != nil {
		return srv.ListenAndServeTLS(userDefinedTlsConfiguration.CertificatePath, userDefinedTlsConfiguration.PrivateKeyPath)
	}

	// if no TLS configuration is present, work in unencrypted mode
	return srv.ListenAndServe()
}

// newHandler returns the handler of the webserver. The router keeps the path encoded, otherwise the escaped slashes
// in names of local disks (/api/%2Fbackup%2Fdb) would be decoded and the path redirected to a cleaned, unknown path.
func newHandler() http.Handler {
	router := mux.NewRouter().UseEncodedPath()

	router.PathPrefix("/").Handler(GetInstance().endpointsRouter)

	return router
}

// strictTLSConfig is used for `strict: true`. The key exchanges are left to Go's defaults,
// which prefer the post-quantum hybrid X25519MLKEM768 and still offer X25519 and the NIST curves.
func strictTLSConfig() *tls.Config {
	return &tls.Config{
		MinVersion: tls.VersionTLS12,
		// only AEAD ciphers with forward secrecy; this list applies to TLS 1.2, TLS 1.3 suites are always secure
		CipherSuites: []uint16{
			tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305_SHA256,
			tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305_SHA256,
			tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
		},
	}
}
