package http

import (
	"net/http"
	"net/http/pprof"
)

type profile struct{}

type httpMux interface {
	HandleFunc(pattern string, handler func(http.ResponseWriter, *http.Request))
}

// Install adds the Profiling webservice to the given mux.
func (p profile) Install(mux httpMux) {
	mux.HandleFunc("/debug/pprof", redirectTo("/debug/pprof/"))
	mux.HandleFunc("/debug/pprof/", http.HandlerFunc(pprof.Index))
	mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
	mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	mux.HandleFunc("/debug/pprof/trace", pprof.Trace)
}

// redirectTo redirects request to a certain destination.
func redirectTo(to string) func(http.ResponseWriter, *http.Request) {
	return func(rw http.ResponseWriter, req *http.Request) {
		http.Redirect(rw, req, to, http.StatusFound)
	}
}
