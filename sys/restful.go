package sys

import (
	"context"
	"net"
	"net/http"
	"time"

	restful "github.com/emicklei/go-restful"
	"github.com/yubo/golib/util"
	"k8s.io/klog/v2"
)

func (p *Module) restfulPrestart() {

	// refresh serveMux
	p.serveMux = http.NewServeMux()
	container := restful.NewContainer()
	container.ServeMux = p.serveMux
	p.restContainer = container

	if p.HttpCross {
		// Add container filter to enable CORS
		cors := restful.CrossOriginResourceSharing{
			AllowedHeaders: []string{"Content-Type",
				"Accept", "x-api-key", "Authorization",
				"x-otp-code", "Referer", "User-Agent:",
				"X-Requested-With", "Origin", "host",
				"Connection", "Accept-Language", "Accept-Encoding",
			},
			AllowedMethods: []string{"GET", "POST", "PUT", "DELETE"},
			CookiesAllowed: true,
			Container:      container,
		}
		container.Filter(cors.Filter)

		// Add container filter to respond to OPTIONS
		container.Filter(container.OPTIONSFilter)
	}
}

func (p *Module) restfulStart() error {

	if util.AddrIsDisable(p.HttpAddr) {
		return nil
	}

	server := &http.Server{
		Addr:    p.HttpAddr,
		Handler: p.serveMux,
	}

	klog.V(5).Infof("ListenAndServe addr %s", p.HttpAddr)

	addr := server.Addr
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}

	go func() {
		server.Serve(tcpKeepAliveListener{ln.(*net.TCPListener)})
	}()

	go func() {
		<-p.ctx.Done()
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*20)
		defer cancel()
		server.Shutdown(ctx)
	}()

	return nil
}

// same as restful.Add()
func (p *Module) RestAdd(service *restful.WebService) {
	p.restContainer.Add(service)
}

func (p *Module) RestFilter(filter restful.FilterFunction) {
	p.restContainer.Filter(filter)
}

// same as http.Handle()
func (p *Module) HttpHandle(pattern string, handler http.Handler) {
	p.serveMux.Handle(pattern, handler)
}

func (p *Module) HttpHandleFunc(pattern string, handler func(http.ResponseWriter, *http.Request)) {
	p.serveMux.HandleFunc(pattern, handler)
}

// tcpKeepAliveListener sets TCP keep-alive timeouts on accepted
// connections. It's used by ListenAndServe and ListenAndServeTLS so
// dead TCP connections (e.g. closing laptop mid-download) eventually
// go away.
type tcpKeepAliveListener struct {
	*net.TCPListener
}

func (ln tcpKeepAliveListener) Accept() (net.Conn, error) {
	tc, err := ln.AcceptTCP()
	if err != nil {
		return nil, err
	}
	tc.SetKeepAlive(true)
	tc.SetKeepAlivePeriod(3 * time.Minute)
	return tc, nil
}
