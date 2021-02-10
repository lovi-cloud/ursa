package gohttpd

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"

	yaml "gopkg.in/yaml.v2"
	sqlite3 "github.com/mattn/go-sqlite3"
	uuid "github.com/satori/go.uuid"

	"go.uber.org/zap"

	"github.com/lovi-cloud/ursa/datastore"
	"github.com/lovi-cloud/ursa/types"
	"github.com/lovi-cloud/ursa/httpd"
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
			g.logger.Error("failed to get uuid", zap.Error(err))
			return
		}
		mac, err := net.ParseMAC(r.URL.Query().Get("mac"))
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			g.logger.Error("failed to get mac", zap.Error(err))
			return
		}
		serial := r.URL.Query().Get("serial")
		product := r.URL.Query().Get("product")
		manufacturer := r.URL.Query().Get("manufacturer")
		err = registerHostIfNotExists(r.Context(), g.ds, types.HardwareAddr(mac), hostID, serial, product, manufacturer)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			g.logger.Error("failed to register host", zap.Error(err))
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
			g.logger.Error("failed to exec template", zap.Error(err))
			return
		}
		return
	})
}

func (g *GoHTTPd) metadataHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		addr, err := net.ResolveTCPAddr("tcp", r.RemoteAddr)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			g.logger.Error("failed to resolve address", zap.Error(err))
			return
		}
		h, err := g.ds.GetHostByAddress(r.Context(), types.IP(addr.IP))
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			g.logger.Error("failed to get host by address", zap.Error(err))
			return
		}
		out := fmt.Sprintf("hostname: %s\n", h.Name)
		w.Write([]byte(out))
	})
}

func (g *GoHTTPd) userdataHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		addr, err := net.ResolveTCPAddr("tcp", r.RemoteAddr)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			g.logger.Error("failed to resolve address", zap.Error(err))
			return
		}
		h, err := g.ds.GetHostByAddress(r.Context(), types.IP(addr.IP))
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			g.logger.Error("failed to get host by address", zap.Error(err))
			return
		}
		l, err := g.ds.GetLeaseByID(r.Context(), h.ServiceLeaseID)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			g.logger.Error("failed to get lease by id", zap.Error(err))
			return
		}
		config := config{
			ManageEtcHosts: true,
			RunCMD: []string{
				"echo \"dash dash/sh boolean false\" | debconf-set-selections",
				"DEBIAN_FRONTEND=noninteractive dpkg-reconfigure dash",
				"echo \"configure system description '$(dmidecode -s system-serial-number)'\" >> /etc/lldpd.conf",
				"systemctl restart lldpd",
				fmt.Sprintf("wget http://%s/static/ursa-bonder -O /tmp/ursa-bonder", r.Host),
				"chmod +x /tmp/ursa-bonder",
				"pkill dhclient",
				fmt.Sprintf("/tmp/ursa-bonder -driver %s -vlan %d -addr %s -mask %s -gw %s -dns %s",
					"e1000e", 1000, l.IPAddress, net.IP(l.Network.Mask), l.Gateway, l.DNSServer),
			},
			FQDN:     h.Name,
			Hostname: h.Name,
		}

		us, err := g.ds.ListUser(r.Context())
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			w.WriteHeader(http.StatusInternalServerError)
			g.logger.Error("failed to list user", zap.Error(err))
			return
		}
		for _, u := range us {
			ks, err := g.ds.ListKeyByUserID(r.Context(), u.ID)
			if errors.Is(err, sql.ErrNoRows) {
				continue
			} else if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				g.logger.Error("failed to list key by user_id", zap.Error(err))
				return
			}
			var keys []string
			for _, k := range ks {
				keys = append(keys, k.Key)
			}
			config.Users = append(config.Users, user{
				Name:              u.Name,
				Sudo:              "ALL=(ALL) NOPASSWD:ALL",
				Groups:            "users, admin",
				SSHAuthorizedKeys: keys,
			})
		}
		out, err := yaml.Marshal(config)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			g.logger.Error("failed to parse yaml", zap.Error(err))
			return
		}
		out = append([]byte("#cloud-config\n"), out...)
		w.Write(out)
	})
}

func registerHostIfNotExists(ctx context.Context, ds datastore.Datastore, mac types.HardwareAddr, hostID uuid.UUID, serial, product, manufacturer string) error {
	var sqliteErr sqlite3.Error

	managementLease, err := ds.GetLeaseFromManagementSubnet(ctx, mac)
	if err != nil {
		return err
	}

	serviceLease, err := ds.CreateLeaseFromServiceSubnet(ctx, mac)
	if errors.As(err, &sqliteErr) && sqliteErr.ExtendedCode == sqlite3.ErrConstraintUnique {
		return nil
	} else if err != nil {
		return err
	}

	_, err = ds.RegisterHost(ctx, hostID, serial, product, manufacturer, serviceLease.ID, managementLease.ID)
	if errors.As(err, &sqliteErr) && sqliteErr.ExtendedCode == sqlite3.ErrConstraintUnique {
		return nil
	} else if err != nil {
		return err
	}

	return nil
}

type config struct {
	ManageEtcHosts bool     `yaml:"manage_etc_hosts"`
	FQDN           string   `yaml:"fqdn"`
	Hostname       string   `yaml:"hostname"`
	Users          []user   `yaml:"users"`
	RunCMD         []string `yaml:"runcmd"`
}

type user struct {
	Name              string   `yaml:"name"`
	Sudo              string   `yaml:"sudo"`
	Groups            string   `yaml:"groups"`
	SSHAuthorizedKeys []string `yaml:"ssh_authorized_keys"`
}
