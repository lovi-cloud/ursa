package gohttpd

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"

	"github.com/whywaita/ursa/types"

	sqlite3 "github.com/mattn/go-sqlite3"

	uuid "github.com/satori/go.uuid"

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
	mux.Handle("/ipxe", g.loggingHandler(g.ipxeHandler()))
	mux.Handle("/static/", g.loggingHandler(http.StripPrefix("/static/", http.FileServer(http.Dir("static")))))
	mux.Handle("/init/meta-data", g.loggingHandler(g.metadataHandler()))
	mux.Handle("/init/user-data", g.loggingHandler(g.userdataHandler()))

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

func (g *GoHTTPd) ipxeHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hostID, err := uuid.FromString(r.URL.Query().Get("uuid"))
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		mac, err := net.ParseMAC(r.URL.Query().Get("mac"))
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		err = registerHostIfNotExists(r.Context(), g.ds, types.HardwareAddr(mac), hostID)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		err = tmpl.Execute(w, ipxeParams{
			Initrd:   fmt.Sprintf("http://%s/static/initrd.img", r.Host),
			Kernel:   fmt.Sprintf("http://%s/static/kernel", r.Host),
			RootFS:   fmt.Sprintf("http://%s/static/filesystem.squashfs", r.Host),
			Metadata: fmt.Sprintf("http://%s/init/", r.Host),
		})
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		return
	})
}

func (g *GoHTTPd) metadataHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tmp := `hostname: cn0001
#instance-action: none
instance-id: i-87018aed
instance-type: isucon.isucon
local-hostname: cn0001.internal.isucon.net
local-ipv4: 192.168.0.100
placement: {availability-zone: japan-01}
public-ipv4: 157.112.67.126
`
		w.Write([]byte(tmp))
	})
}

func (g *GoHTTPd) userdataHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tmp := `#cloud-config
`
		w.Write([]byte(tmp))
	})
}

func registerHostIfNotExists(ctx context.Context, ds datastore.Datastore, mac types.HardwareAddr, hostID uuid.UUID) error {
	var sqliteErr sqlite3.Error

	lease, err := ds.CreateLeaseFromServiceSubnet(ctx, mac)
	if errors.As(err, &sqliteErr) && sqliteErr.ExtendedCode == sqlite3.ErrConstraintUnique {
		return nil
	} else if err != nil {
		return err
	}

	_, err = ds.RegisterHost(ctx, hostID, lease.ID)
	if errors.As(err, &sqliteErr) && sqliteErr.ExtendedCode == sqlite3.ErrConstraintUnique {
		return nil
	} else if err != nil {
		return err
	}

	return nil
}
