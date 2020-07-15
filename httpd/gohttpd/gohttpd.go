package gohttpd

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"

	"go.uber.org/zap"

	"github.com/whywaita/ursa/datastore"
	"github.com/whywaita/ursa/httpd"
)

// GoHTTPd is
type GoHTTPd struct {
	ds     datastore.Datastore
	logger *zap.Logger
}

// New is
func New(ds datastore.Datastore, logger *zap.Logger) (httpd.HTTPd, error) {
	return &GoHTTPd{
		ds:     ds,
		logger: logger,
	}, nil
}

// Serve is
func (g *GoHTTPd) Serve(ctx context.Context, addr string) error {
	mux := http.NewServeMux()
	mux.Handle("/", g.loggingHandler(http.NotFoundHandler()))
	mux.Handle("/ipxe/", g.loggingHandler(g))
	mux.Handle("/static/", g.loggingHandler(http.StripPrefix("/static/", http.FileServer(http.Dir("static")))))

	return http.ListenAndServe(addr, mux)
}

func (g *GoHTTPd) loggingHandler(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		g.logger.Info("http request log", zap.String("url", r.URL.String()), zap.String("remote", r.RemoteAddr))
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, r)
		g.logger.Info("http response log", zap.Int("code", rec.Code))
		w.WriteHeader(rec.Code)
		rec.Body.WriteTo(w)
	})
}

func (g *GoHTTPd) ServeHTTP(resp http.ResponseWriter, req *http.Request) {
	g.logger.Info("call", zap.String("path", req.RequestURI))
	err := tmpl.Execute(resp, ipxeParams{
		Initrd: fmt.Sprintf("http://%s/static/initrd.img", req.Host),
		Kernel: fmt.Sprintf("http://%s/static/kernel", req.Host),
		RootFS: fmt.Sprintf("http://%s/static/filesystem.squashfs", req.Host),
	})
	if err != nil {
		resp.WriteHeader(http.StatusBadRequest)
		return
	}
	return
}
