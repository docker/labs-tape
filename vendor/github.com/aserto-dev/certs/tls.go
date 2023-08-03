package certs

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"

	"github.com/pkg/errors"
	"google.golang.org/grpc/credentials"
)

// TLSCredsConfig contains paths to certificates
type TLSCredsConfig struct {
	TLSCertPath   string `json:"tls_cert_path"`
	TLSKeyPath    string `json:"tls_key_path"`
	TLSCACertPath string `json:"tls_ca_cert_path"`
}

// GRPCServerTLSCreds gets TLS credentials for a GRPC server
func GRPCServerTLSCreds(config TLSCredsConfig) (credentials.TransportCredentials, error) {
	certificate, err := tls.LoadX509KeyPair(config.TLSCertPath, config.TLSKeyPath)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to load GRPC certs")
	}

	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{certificate},
		MinVersion:   tls.VersionTLS12,
	}

	return credentials.NewTLS(tlsConfig), nil
}

// GatewayAsClientTLSCreds returns transport credentials so an HTTP gateway can connect to the GRPC server
func GatewayAsClientTLSCreds(config TLSCredsConfig) (credentials.TransportCredentials, error) {

	certPool := x509.NewCertPool()
	caCertBytes, err := os.ReadFile(config.TLSCACertPath)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to read ca cert: %s", config.TLSCACertPath)
	}

	ok := certPool.AppendCertsFromPEM(caCertBytes)
	if !ok {
		return nil, errors.Wrap(err, "failed to append client ca cert: %s")
	}

	certificate, err := tls.LoadX509KeyPair(config.TLSCertPath, config.TLSKeyPath)
	if err != nil {
		return nil, fmt.Errorf("could not load server key pair: %s", err)
	}

	clientCreds := credentials.NewTLS(&tls.Config{
		Certificates: []tls.Certificate{certificate},
		RootCAs:      certPool,
		MinVersion:   tls.VersionTLS12,
	})

	return clientCreds, nil
}

// GatewayServerTLSConfig returns a TLS config for the gateway server
func GatewayServerTLSConfig(config TLSCredsConfig) (*tls.Config, error) {
	certificate, err := tls.LoadX509KeyPair(config.TLSCertPath, config.TLSKeyPath)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to load gateway certs")
	}

	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{certificate},
		MinVersion:   tls.VersionTLS12,
	}

	return tlsConfig, nil
}
