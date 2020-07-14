package gohttpd

import (
	"context"
	"fmt"
	"net/http"

	"go.uber.org/zap"

	"github.com/whywaita/ursa/datastore"
	"github.com/whywaita/ursa/httpd"
)

// GoHTTPd is
type GoHTTPd struct {
	addr   string
	ds     datastore.Datastore
	logger *zap.Logger
}

// New is
func New(addr string, ds datastore.Datastore, logger *zap.Logger) (httpd.HTTPd, error) {
	return &GoHTTPd{
		addr:   addr,
		ds:     ds,
		logger: logger,
	}, nil
}

// Serve is
func (g *GoHTTPd) Serve(ctx context.Context) error {
	mux := http.NewServeMux()
	mux.Handle("/ipxe/", g)
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))

	return http.ListenAndServe(g.addr, mux)
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
