package main

import (
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	"github.com/rafaelmartins/distfiles/internal/settings"
	"github.com/rafaelmartins/distfiles/internal/views"
)

func usage(err error) {
	fmt.Fprintln(os.Stderr, "usage: distfiles")
	if err != nil {
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "error:", err.Error())
		os.Exit(1)
	}
	os.Exit(0)
}

func logHandler(w io.Writer, params handlers.LogFormatterParams) {
	uri := params.Request.RequestURI
	if params.Request.ProtoMajor == 2 && params.Request.Method == "CONNECT" {
		uri = params.Request.Host
	}
	if uri == "" {
		uri = params.URL.RequestURI()
	}

	fmt.Fprintf(w, "[%s] %s %q %d %d\n",
		params.TimeStamp.UTC().Format("2006-01-02 15:04:05 MST"),
		params.Request.Method,
		uri,
		params.StatusCode,
		params.Size,
	)
}

func main() {
	s, err := settings.Get()
	if err != nil {
		usage(err)
	}

	r := mux.NewRouter()
	r.HandleFunc("/", views.Upload).Methods("POST")
	r.HandleFunc("/", views.Health)

	h := handlers.CustomLoggingHandler(os.Stderr, r, logHandler)
	h = handlers.RecoveryHandler(handlers.PrintRecoveryStack(true))(h)

	fmt.Fprintf(os.Stderr, " * Listening on %s\n", s.ListenAddr)
	if err := http.ListenAndServe(s.ListenAddr, h); err != nil {
		usage(err)
	}
}
