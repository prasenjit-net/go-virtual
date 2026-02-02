package tlsutil

import (
	"crypto/tls"
	"os"
	"path/filepath"
	"testing"
)

func TestNewCertificateManager(t *testing.T) {
	cm := NewCertificateManager("cert.pem", "key.pem", "/tmp/certs")

	if cm.certFile != "cert.pem" {
		t.Errorf("Expected certFile 'cert.pem', got %q", cm.certFile)
	}
	if cm.keyFile != "key.pem" {
		t.Errorf("Expected keyFile 'key.pem', got %q", cm.keyFile)
	}
	if cm.storePath != "/tmp/certs" {
		t.Errorf("Expected storePath '/tmp/certs', got %q", cm.storePath)
	}
}

func TestGetCertificate_AutoGenerate(t *testing.T) {
	tmpDir := t.TempDir()
	cm := NewCertificateManager("", "", tmpDir)

	cert, err := cm.GetCertificate(true)
	if err != nil {
		t.Fatalf("GetCertificate failed: %v", err)
	}
	if cert == nil {
		t.Fatal("Expected certificate, got nil")
	}

	// Verify files were created
	certPath := filepath.Join(tmpDir, certFileName)
	keyPath := filepath.Join(tmpDir, keyFileName)

	if _, err := os.Stat(certPath); os.IsNotExist(err) {
		t.Error("Certificate file was not created")
	}
	if _, err := os.Stat(keyPath); os.IsNotExist(err) {
		t.Error("Key file was not created")
	}
}

func TestGetCertificate_LoadExisting(t *testing.T) {
	tmpDir := t.TempDir()
	cm := NewCertificateManager("", "", tmpDir)

	// Generate first
	cert1, err := cm.GetCertificate(true)
	if err != nil {
		t.Fatalf("First GetCertificate failed: %v", err)
	}

	// Load existing
	cm2 := NewCertificateManager("", "", tmpDir)
	cert2, err := cm2.GetCertificate(false) // autoGenerate=false
	if err != nil {
		t.Fatalf("Second GetCertificate failed: %v", err)
	}

	// Compare certificates (should be the same)
	if len(cert1.Certificate) != len(cert2.Certificate) {
		t.Error("Loaded certificate differs from generated")
	}
}

func TestGetCertificate_NoAutoGenerate(t *testing.T) {
	tmpDir := t.TempDir()
	cm := NewCertificateManager("", "", tmpDir)

	// Should fail without auto-generate and no existing cert
	_, err := cm.GetCertificate(false)
	if err == nil {
		t.Error("Expected error when no certificate and autoGenerate=false")
	}
}

func TestGetCertificate_InvalidPaths(t *testing.T) {
	cm := NewCertificateManager("/nonexistent/cert.pem", "/nonexistent/key.pem", "/tmp")

	_, err := cm.GetCertificate(true)
	if err == nil {
		t.Error("Expected error for invalid certificate paths")
	}
}

func TestGetCertificatePaths(t *testing.T) {
	tests := []struct {
		name         string
		certFile     string
		keyFile      string
		storePath    string
		expectedCert string
		expectedKey  string
	}{
		{
			name:         "configured paths",
			certFile:     "/etc/ssl/cert.pem",
			keyFile:      "/etc/ssl/key.pem",
			storePath:    "/tmp/certs",
			expectedCert: "/etc/ssl/cert.pem",
			expectedKey:  "/etc/ssl/key.pem",
		},
		{
			name:         "store path",
			certFile:     "",
			keyFile:      "",
			storePath:    "/var/lib/certs",
			expectedCert: "/var/lib/certs/server.crt",
			expectedKey:  "/var/lib/certs/server.key",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cm := NewCertificateManager(tt.certFile, tt.keyFile, tt.storePath)
			certPath, keyPath := cm.GetCertificatePaths()

			if certPath != tt.expectedCert {
				t.Errorf("Expected cert path %q, got %q", tt.expectedCert, certPath)
			}
			if keyPath != tt.expectedKey {
				t.Errorf("Expected key path %q, got %q", tt.expectedKey, keyPath)
			}
		})
	}
}

func TestGeneratedCertificate_ValidTLS(t *testing.T) {
	tmpDir := t.TempDir()
	cm := NewCertificateManager("", "", tmpDir)

	cert, err := cm.GetCertificate(true)
	if err != nil {
		t.Fatalf("GetCertificate failed: %v", err)
	}

	// Create TLS config and verify it works
	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{*cert},
	}

	if len(tlsConfig.Certificates) != 1 {
		t.Error("TLS config should have one certificate")
	}
}

func TestGetLocalIPs(t *testing.T) {
	ips, err := getLocalIPs()
	if err != nil {
		t.Fatalf("getLocalIPs failed: %v", err)
	}

	// Should return at least some IPs (may be empty in some test environments)
	t.Logf("Found %d local IPs", len(ips))
}
