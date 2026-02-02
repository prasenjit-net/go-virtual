package tlsutil

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"time"
)

const (
	certFileName = "server.crt"
	keyFileName  = "server.key"
)

// CertificateManager handles TLS certificate loading and generation
type CertificateManager struct {
	certFile  string
	keyFile   string
	storePath string
}

// NewCertificateManager creates a new certificate manager
func NewCertificateManager(certFile, keyFile, storePath string) *CertificateManager {
	return &CertificateManager{
		certFile:  certFile,
		keyFile:   keyFile,
		storePath: storePath,
	}
}

// GetCertificate returns a TLS certificate, loading from files or generating if needed
func (cm *CertificateManager) GetCertificate(autoGenerate bool) (*tls.Certificate, error) {
	// Try loading from configured paths first
	if cm.certFile != "" && cm.keyFile != "" {
		cert, err := tls.LoadX509KeyPair(cm.certFile, cm.keyFile)
		if err == nil {
			return &cert, nil
		}
		// If files are specified but can't be loaded, return error
		return nil, fmt.Errorf("failed to load certificate from %s and %s: %w", cm.certFile, cm.keyFile, err)
	}

	// Try loading from store path
	storeCertPath := filepath.Join(cm.storePath, certFileName)
	storeKeyPath := filepath.Join(cm.storePath, keyFileName)

	cert, err := tls.LoadX509KeyPair(storeCertPath, storeKeyPath)
	if err == nil {
		return &cert, nil
	}

	// If auto-generate is disabled and no cert found, return error
	if !autoGenerate {
		return nil, fmt.Errorf("no TLS certificate found and auto-generation is disabled")
	}

	// Generate new self-signed certificate
	return cm.generateAndSaveCertificate()
}

// generateAndSaveCertificate creates a new self-signed certificate and saves it
func (cm *CertificateManager) generateAndSaveCertificate() (*tls.Certificate, error) {
	// Ensure store path exists
	if err := os.MkdirAll(cm.storePath, 0700); err != nil {
		return nil, fmt.Errorf("failed to create certificate store directory: %w", err)
	}

	// Generate ECDSA private key
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("failed to generate private key: %w", err)
	}

	// Generate serial number
	serialNumber, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return nil, fmt.Errorf("failed to generate serial number: %w", err)
	}

	// Create certificate template
	notBefore := time.Now()
	notAfter := notBefore.Add(365 * 24 * time.Hour) // Valid for 1 year

	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{"Go-Virtual"},
			CommonName:   "Go-Virtual Self-Signed",
		},
		NotBefore:             notBefore,
		NotAfter:              notAfter,
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		DNSNames:              []string{"localhost"},
		IPAddresses:           []net.IP{net.ParseIP("127.0.0.1"), net.ParseIP("::1")},
	}

	// Add all local IPs
	localIPs, err := getLocalIPs()
	if err == nil {
		template.IPAddresses = append(template.IPAddresses, localIPs...)
	}

	// Create certificate
	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &privateKey.PublicKey, privateKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create certificate: %w", err)
	}

	// Encode certificate to PEM
	certPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: certDER,
	})

	// Encode private key to PEM
	keyDER, err := x509.MarshalECPrivateKey(privateKey)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal private key: %w", err)
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "EC PRIVATE KEY",
		Bytes: keyDER,
	})

	// Save certificate and key files
	certPath := filepath.Join(cm.storePath, certFileName)
	keyPath := filepath.Join(cm.storePath, keyFileName)

	if err := os.WriteFile(certPath, certPEM, 0644); err != nil {
		return nil, fmt.Errorf("failed to save certificate: %w", err)
	}

	if err := os.WriteFile(keyPath, keyPEM, 0600); err != nil {
		return nil, fmt.Errorf("failed to save private key: %w", err)
	}

	// Parse and return the certificate
	cert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return nil, fmt.Errorf("failed to parse generated certificate: %w", err)
	}

	return &cert, nil
}

// getLocalIPs returns all non-loopback local IP addresses
func getLocalIPs() ([]net.IP, error) {
	var ips []net.IP

	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return nil, err
	}

	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			ips = append(ips, ipnet.IP)
		}
	}

	return ips, nil
}

// GetCertificatePaths returns the paths where certificates are stored
func (cm *CertificateManager) GetCertificatePaths() (certPath, keyPath string) {
	if cm.certFile != "" && cm.keyFile != "" {
		return cm.certFile, cm.keyFile
	}
	return filepath.Join(cm.storePath, certFileName), filepath.Join(cm.storePath, keyFileName)
}
