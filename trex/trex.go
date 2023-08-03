package trex

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/aserto-dev/certs"
	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/uuid"
	"github.com/rs/zerolog"

	_ "github.com/distribution/distribution/v3/registry/auth/htpasswd"
	_ "github.com/distribution/distribution/v3/registry/auth/silly"
	_ "github.com/distribution/distribution/v3/registry/auth/token"
	_ "github.com/distribution/distribution/v3/registry/proxy"
	_ "github.com/distribution/distribution/v3/registry/storage/driver/filesystem"
	_ "github.com/distribution/distribution/v3/registry/storage/driver/inmemory"
	_ "github.com/distribution/distribution/v3/registry/storage/driver/middleware/redirect"

	"github.com/distribution/distribution/v3/configuration"
	"github.com/distribution/distribution/v3/registry"
)

type Trex struct {
	port                    int
	caCert, tlsKey, tlsCert string

	caPool *x509.CertPool
}

func New(port int) *Trex {
	return &Trex{
		port:   port,
		caPool: x509.NewCertPool(),
	}
}

var Shared = struct {
	*Trex
	*sync.Once
}{
	Trex: New(0),
	Once: &sync.Once{},
}

func RunShared() {
	Shared.Once.Do(func() {
		go func() {
			err := Shared.Run(context.Background())
			if err != nil {
				panic(err)
			}
		}()
	})
	for {
		_, err := (&net.Dialer{Timeout: 2 * time.Second}).
			DialContext(context.Background(), "tcp", Shared.Addr())
		if err == nil {
			break
		}
	}
}

func (r *Trex) Run(ctx context.Context) error {
	if r.port == 0 {
		// automatically allocate the port, and use it for the registry;
		// albeit this can be racy, since registry cannot take a listener
		// and copying the code here is not worth it
		l, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			return err
		}
		r.port = l.Addr().(*net.TCPAddr).Port
		if err := l.Close(); err != nil {
			return err
		}
	}
	// TODO: return a channel to indicate when the server is ready, or perhpas
	// just copy what RunShared does and start a goroutine?
	pkiDir, err := os.MkdirTemp("", "trex-pki-*")
	if err != nil {
		return err
	}

	defer os.RemoveAll(pkiDir)

	r.caCert = filepath.Join(pkiDir, "ca.crt")
	r.tlsKey = filepath.Join(pkiDir, "tls.key")
	r.tlsCert = filepath.Join(pkiDir, "tls.crt")

	if err := certs.NewGenerator(zerolog.Ctx(ctx)).
		MakeDevCert(&certs.CertGenConfig{
			CommonName:  "localhost",
			CACertPath:  r.caCert,
			CertKeyPath: r.tlsKey,
			CertPath:    r.tlsCert,
		}); err != nil {
		return err
	}

	caCert, err := os.ReadFile(r.caCert)
	if err != nil {
		return err
	}

	if !r.caPool.AppendCertsFromPEM(caCert) {
		return fmt.Errorf("failed to setup CA certificate pool")
	}

	config := &configuration.Configuration{
		Storage: configuration.Storage{
			"inmemory": configuration.Parameters{},
			"delete":   configuration.Parameters{"enabled": false},
			"maintenance": configuration.Parameters{
				"uploadpurging": map[interface{}]interface{}{
					"enabled": false,
				}},
		},
		Catalog: configuration.Catalog{
			MaxEntries: 100,
		},
	}
	config.HTTP.Addr = r.Addr()
	config.HTTP.TLS.Certificate = r.tlsCert
	config.HTTP.TLS.Key = r.tlsKey

	registry, err := registry.NewRegistry(ctx, config)
	if err != nil {
		return err
	}

	if err = registry.ListenAndServe(); err != nil {
		return err
	}
	return nil
}

func (r *Trex) Addr() string {
	return fmt.Sprintf("127.0.0.1:%d", r.port)
}

func (r *Trex) CraneOptions() []crane.Option {
	transport := remote.DefaultTransport.(*http.Transport).Clone()

	transport.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
		if addr != r.Addr() {
			return remote.DefaultTransport.(*http.Transport).DialContext(ctx, network, addr)
		}

		dialer := tls.Dialer{
			NetDialer: &net.Dialer{
				Timeout:   30 * time.Second,
				KeepAlive: 30 * time.Second,
			},
			Config: &tls.Config{
				RootCAs: r.caPool,
			},
		}

		return dialer.DialContext(ctx, network, addr)
	}

	return []crane.Option{
		crane.WithTransport(transport),
	}
}

func (r *Trex) NewUniqueRepoNamer(knownInfix string) func(string) string {
	uniqueInfix := uuid.New().String()
	return func(name string) string {
		return fmt.Sprintf("%s/%s/%s/%s",
			r.Addr(), uniqueInfix, knownInfix, name)
	}
}
