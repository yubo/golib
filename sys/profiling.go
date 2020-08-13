package sys

import (
	"net/http"
	"net/http/pprof"
)

// Install adds the Profiling webservice to the given mux.
func (p *Module) InstallProfiling() {
	p.HttpHandleFunc("/debug/pprof", redirectTo("/debug/pprof/"))
	p.HttpHandleFunc("/debug/pprof/", http.HandlerFunc(pprof.Index))
	p.HttpHandleFunc("/debug/pprof/profile", pprof.Profile)
	p.HttpHandleFunc("/debug/pprof/symbol", pprof.Symbol)
	p.HttpHandleFunc("/debug/pprof/trace", pprof.Trace)
}

// redirectTo redirects request to a certain destination.
func redirectTo(to string) func(http.ResponseWriter, *http.Request) {
	return func(rw http.ResponseWriter, req *http.Request) {
		http.Redirect(rw, req, to, http.StatusFound)
	}
}
