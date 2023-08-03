package certs

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"time"

	"github.com/pkg/errors"
	"github.com/rs/zerolog"
)

// Generator generates certs without any external dependencies
type Generator struct {
	logger *zerolog.Logger
}

// CertGenConfig contains details about how cert generation should happen
type CertGenConfig struct {
	CommonName       string
	CertKeyPath      string
	CertPath         string
	CACertPath       string
	DefaultTLSGenDir string
}

// NewGenerator creates a new cert generator
func NewGenerator(logger *zerolog.Logger) *Generator {
	log := logger.With().Str("component", "cert-generator").Logger()
	return &Generator{
		logger: &log,
	}
}

// MakeDevCert creates a development certificate request and private key.
// It persists it in the work dir and returns the CSR.
func (c *Generator) MakeDevCert(genConfig *CertGenConfig) error {
	c.logger.Info().
		Str("common-name", genConfig.CommonName).
		Str("cert-path", genConfig.CertPath).
		Str("key-path", genConfig.CertKeyPath).
		Str("ca-cert-path", genConfig.CACertPath).
		Msg("generating certificate")

	err := c.checkDir(genConfig)
	if err != nil {
		return errors.Wrap(err, "directory verification returned an error")
	}

	c.logger.Info().Str("file", genConfig.CACertPath).Str("common-name", genConfig.CommonName).Msg("generating ca certificate")

	ca := &x509.Certificate{
		SerialNumber: big.NewInt(2019),
		Subject: pkix.Name{
			Organization:  []string{"Aserto, Inc."},
			Country:       []string{"US"},
			Province:      []string{"WA"},
			Locality:      []string{"Seattle"},
			StreetAddress: []string{"-"},
			PostalCode:    []string{"-"},
			CommonName:    genConfig.CommonName + "-ca",
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().AddDate(1, 0, 0),
		IsCA:                  true,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
	}

	caPrivKey, err := rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		return err
	}
	caBytes, err := x509.CreateCertificate(rand.Reader, ca, ca, &caPrivKey.PublicKey, caPrivKey)
	if err != nil {
		return err
	}

	caPEM := new(bytes.Buffer)
	err = pem.Encode(caPEM, &pem.Block{
		Type:  "CERTIFICATE",
		Bytes: caBytes,
	})
	if err != nil {
		return errors.Wrap(err, "failed to encode cert")
	}

	caPrivKeyPEM := new(bytes.Buffer)
	err = pem.Encode(caPrivKeyPEM, &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(caPrivKey),
	})
	if err != nil {
		return errors.Wrap(err, "failed to encode key")
	}

	c.logger.Info().Str("common-name", genConfig.CommonName).Msg("generating certificate and key")

	cert := &x509.Certificate{
		SerialNumber: big.NewInt(1658),
		Subject: pkix.Name{
			Organization:  []string{"Aserto, Inc."},
			Country:       []string{"US"},
			Province:      []string{"WA"},
			Locality:      []string{"Seattle"},
			StreetAddress: []string{"-"},
			PostalCode:    []string{"-"},
			CommonName:    genConfig.CommonName,
		},
		IPAddresses:  []net.IP{net.IPv4(0, 0, 0, 0), net.IPv4(127, 0, 0, 1), net.IPv6loopback},
		DNSNames:     []string{"localhost"},
		NotBefore:    time.Now(),
		NotAfter:     time.Now().AddDate(1, 0, 0),
		SubjectKeyId: []byte{1, 2, 3, 4, 6},
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		KeyUsage:     x509.KeyUsageDigitalSignature,
	}

	certPrivKey, err := rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		return err
	}

	c.logger.Info().Str("cert-file", genConfig.CertPath).Str("key-file", genConfig.CertKeyPath).Str("common-name", genConfig.CommonName).Msg("signing certificate")

	return c.signCerts(genConfig, cert, ca, certPrivKey, caPrivKey, caPEM)
}

func (c *Generator) signCerts(genConfig *CertGenConfig, cert, ca *x509.Certificate, certPrivKey, caPrivKey *rsa.PrivateKey, caPEM *bytes.Buffer) error {
	c.logger.Info().Str("cert-file", genConfig.CertPath).Str("key-file", genConfig.CertKeyPath).Str("common-name", genConfig.CommonName).Msg("signing certificate")

	certBytes, err := x509.CreateCertificate(rand.Reader, cert, ca, &certPrivKey.PublicKey, caPrivKey)
	if err != nil {
		return err
	}

	certPEM := new(bytes.Buffer)
	err = pem.Encode(certPEM, &pem.Block{
		Type:  "CERTIFICATE",
		Bytes: certBytes,
	})
	if err != nil {
		return errors.Wrap(err, "failed to encode cert")
	}

	certPrivKeyPEM := new(bytes.Buffer)
	err = pem.Encode(certPrivKeyPEM, &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(certPrivKey),
	})
	if err != nil {
		return errors.Wrap(err, "failed to encode key")
	}

	err = writeFile(
		genConfig.CACertPath,
		caPEM.Bytes(),
	)
	if err != nil {
		return errors.Wrap(err, "failed to write ca cert")
	}

	err = writeFile(
		genConfig.CertKeyPath,
		certPrivKeyPEM.Bytes(),
	)
	if err != nil {
		return errors.Wrap(err, "failed to write key")
	}

	err = writeFile(
		genConfig.CertKeyPath,
		certPrivKeyPEM.Bytes(),
	)
	if err != nil {
		return errors.Wrap(err, "failed to write key")
	}

	err = writeFile(
		genConfig.CertPath,
		certPEM.Bytes(),
	)
	if err != nil {
		return errors.Wrap(err, "failed to write key")
	}

	return nil
}

func writeFile(file string, contents []byte) error {
	fo, err := os.Create(file)
	if err != nil {
		return errors.Wrapf(err, "failed to open cert file '%s' for writing", file)
	}
	defer func() {
		err = fo.Close()
		if err != nil {
			err = errors.Wrapf(err, "failed to close cert file '%s'", file)
		}
	}()

	_, err = fo.Write(contents)
	if err != nil {
		return errors.Wrapf(err, "failed to write cert contents to file '%s'", file)
	}

	return err
}

func (c *Generator) checkDir(genConfig *CertGenConfig) error {

	certDir := filepath.Dir(genConfig.CertPath)
	keyDir := filepath.Dir(genConfig.CertKeyPath)
	caCertDir := filepath.Dir(genConfig.CACertPath)

	if certDir != keyDir || certDir != caCertDir {
		return errors.New("output directory for all configured certificates and keys must be the same")
	}

	if certDir == genConfig.DefaultTLSGenDir {
		err := os.MkdirAll(certDir, 0777)
		if err != nil {
			return errors.Wrapf(err, "failed to create directory '%s'", genConfig.DefaultTLSGenDir)
		}
	}

	if _, err := os.Stat(certDir); os.IsNotExist(err) {
		return errors.Errorf("output directory '%s' doesn't exist", certDir)
	} else if err != nil {
		return errors.Wrapf(err, "failed to determine if output directory '%s' exists", certDir)
	}

	return nil
}
