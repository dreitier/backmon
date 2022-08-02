package web

import (
	"github.com/dreitier/cloudmon/config"
	"fmt"
	"github.com/gorilla/mux"
	log "github.com/sirupsen/logrus"
	"net/http"
)

func StartServer() {
	port := config.GetInstance().Global().HttpPort()
	listenAddr := fmt.Sprintf(":%d", port)

	log.Infof("Starting webserver on %s", listenAddr)
	
	r := mux.NewRouter()
	//r.HandleFunc("/{disk}", handler)

	r.PathPrefix("/").Handler(GetInstance().endpointsRouter)

	srv := &http.Server{
		Handler: r,
		Addr:    listenAddr,
		// Good practice: enforce timeouts for servers you create!
		//WriteTimeout: 15 * time.Second,
		//ReadTimeout:  15 * time.Second,
	}

	log.Fatal(srv.ListenAndServe())
	log.Infof("Listening on %s", listenAddr)
}
