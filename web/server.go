package web

import (
	"github.com/dreitier/cloudmon/config"
	"fmt"
	"github.com/gorilla/mux"
	log "github.com/sirupsen/logrus"
	"net/http"
	"crypto/tls"
)

func StartServer() {
	port := config.GetInstance().Global().HttpPort()
	listenAddr := fmt.Sprintf(":%d", port)

	log.Infof("Starting webserver on %s", listenAddr)
	
	// #11: support for TLS configuration
	// optional tls.Config
	var tlsServerConfig*tls.Config = nil
	// by default, we are not configuring TLS ciphers and let them as it is
	var tlsNextProto map[string]func(*http.Server, *tls.Conn, http.Handler) = nil
	// get user-defined TLS configuration from config.yaml
	userDefinedTlsConfiguration := config.GetInstance().Http().Tls

	if userDefinedTlsConfiguration != nil {
		// `strict: true` sets the TLS configuration to something SSLLabs prefers
		// @see https://gist.github.com/denji/12b3a568f092ab951456
		if (userDefinedTlsConfiguration.IsStrict) {
			tlsServerConfig = &tls.Config{
				MinVersion:               tls.VersionTLS12,
				CurvePreferences:         []tls.CurveID{tls.CurveP521, tls.CurveP384, tls.CurveP256},
				PreferServerCipherSuites: true,
				CipherSuites: []uint16{
					tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
					tls.TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA,
					tls.TLS_RSA_WITH_AES_256_GCM_SHA384,
					tls.TLS_RSA_WITH_AES_256_CBC_SHA,
				},
			}
		}

		// provide an empty hashmap to disable any other TLS ciphers
		restrictedProtos := make(map[string]func(*http.Server, *tls.Conn, http.Handler), 0)
		tlsNextProto = restrictedProtos
	}

	router := mux.NewRouter()

	router.PathPrefix("/").Handler(GetInstance().endpointsRouter)

	srv := &http.Server{
		Handler:      router,
		Addr:         listenAddr,
		TLSConfig:    tlsServerConfig,
		TLSNextProto: tlsNextProto,
	}

	// if the user has provided a TLS configuration, start with TLS
	if userDefinedTlsConfiguration != nil {
		log.Error(srv.ListenAndServeTLS(userDefinedTlsConfiguration.CertificatePath, userDefinedTlsConfiguration.PrivateKeyPath))
	} else {
		// if no TLS configuration is present, work in unecrypted mode
		log.Error(srv.ListenAndServe())
	}
}
