package proxy

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net/http"
	"os"
	"time"
)

// ClientConfig holds the HTTP client settings used by the proxy recorder when
// forwarding requests to a backend in proxy/recording mode.
type ClientConfig struct {
	// TimeoutSeconds is the overall request timeout. Defaults to 30 s when ≤ 0.
	TimeoutSeconds int

	// InsecureSkipVerify disables TLS server certificate verification.
	// Set to true only for development backends that use self-signed certs.
	InsecureSkipVerify bool

	// CertFile and KeyFile are PEM-encoded client certificate and private key
	// paths. Both must be set together to enable mutual TLS.
	CertFile string
	KeyFile  string

	// CACertFile is an optional PEM-encoded CA certificate used to verify the
	// backend server's TLS certificate. When empty the system CA pool is used.
	CACertFile string
}

// BuildClient constructs an *http.Client from cfg.
//
//   - If CertFile + KeyFile are provided the client presents a certificate to
//     the backend (mTLS / mutual TLS).
//   - If CACertFile is provided it replaces the system root CA pool used to
//     verify the backend's server certificate.
//   - InsecureSkipVerify=true disables server certificate verification entirely
//     (useful for development backends with self-signed certs; never use in
//     production).
func BuildClient(cfg ClientConfig) (*http.Client, error) {
	tlsCfg := &tls.Config{
		InsecureSkipVerify: cfg.InsecureSkipVerify, //nolint:gosec
	}

	// ── Client certificate (mTLS) ─────────────────────────────────────────────
	if cfg.CertFile != "" || cfg.KeyFile != "" {
		if cfg.CertFile == "" || cfg.KeyFile == "" {
			return nil, fmt.Errorf("proxy: mtls requires both certFile and keyFile to be set")
		}
		cert, err := tls.LoadX509KeyPair(cfg.CertFile, cfg.KeyFile)
		if err != nil {
			return nil, fmt.Errorf("proxy: failed to load client certificate (%s, %s): %w",
				cfg.CertFile, cfg.KeyFile, err)
		}
		tlsCfg.Certificates = []tls.Certificate{cert}
	}

	// ── Custom CA pool ────────────────────────────────────────────────────────
	if cfg.CACertFile != "" {
		caPEM, err := os.ReadFile(cfg.CACertFile)
		if err != nil {
			return nil, fmt.Errorf("proxy: failed to read CA certificate %s: %w", cfg.CACertFile, err)
		}
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(caPEM) {
			return nil, fmt.Errorf("proxy: no valid PEM certificates found in CA file %s", cfg.CACertFile)
		}
		tlsCfg.RootCAs = pool
	}

	timeout := time.Duration(cfg.TimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = 30 * time.Second
	}

	transport := &http.Transport{
		TLSClientConfig: tlsCfg,
	}

	return &http.Client{
		Transport: transport,
		Timeout:   timeout,
	}, nil
}
