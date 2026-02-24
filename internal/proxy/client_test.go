package proxy

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/prasenjit/go-virtual/internal/models"
)

// generateSelfSignedCert writes a PEM-encoded self-signed certificate and its
// private key into dir, returning the paths (certFile, keyFile, certPEM).
func generateSelfSignedCert(t *testing.T, dir string) (certFile, keyFile string, certPEM []byte) {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}

	serial, _ := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	tmpl := &x509.Certificate{
		SerialNumber: serial,
		Subject:      pkix.Name{CommonName: "test"},
		NotBefore:    time.Now().Add(-time.Minute),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
	}

	certDER, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("create cert: %v", err)
	}

	certPEM = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})

	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		t.Fatalf("marshal key: %v", err)
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})

	certFile = filepath.Join(dir, "client.crt")
	keyFile = filepath.Join(dir, "client.key")
	if err := os.WriteFile(certFile, certPEM, 0644); err != nil {
		t.Fatalf("write cert: %v", err)
	}
	if err := os.WriteFile(keyFile, keyPEM, 0600); err != nil {
		t.Fatalf("write key: %v", err)
	}
	return certFile, keyFile, certPEM
}

// ---- BuildClient ----

func TestBuildClient_Defaults(t *testing.T) {
	client, err := BuildClient(ClientConfig{TimeoutSeconds: 5})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if client == nil {
		t.Fatal("expected non-nil client")
	}
	if client.Timeout != 5*time.Second {
		t.Errorf("expected timeout 5s, got %v", client.Timeout)
	}
}

func TestBuildClient_DefaultTimeout_WhenZero(t *testing.T) {
	client, err := BuildClient(ClientConfig{TimeoutSeconds: 0})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if client.Timeout != 30*time.Second {
		t.Errorf("expected default 30s timeout, got %v", client.Timeout)
	}
}

func TestBuildClient_DefaultTimeout_WhenNegative(t *testing.T) {
	client, err := BuildClient(ClientConfig{TimeoutSeconds: -1})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if client.Timeout != 30*time.Second {
		t.Errorf("expected default 30s timeout for negative, got %v", client.Timeout)
	}
}

func TestBuildClient_InsecureSkipVerify(t *testing.T) {
	client, err := BuildClient(ClientConfig{InsecureSkipVerify: true, TimeoutSeconds: 5})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	tr, ok := client.Transport.(*http.Transport)
	if !ok {
		t.Fatal("expected *http.Transport")
	}
	if !tr.TLSClientConfig.InsecureSkipVerify {
		t.Error("expected InsecureSkipVerify=true")
	}
}

func TestBuildClient_InsecureSkipVerify_False(t *testing.T) {
	client, err := BuildClient(ClientConfig{InsecureSkipVerify: false, TimeoutSeconds: 5})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	tr := client.Transport.(*http.Transport)
	if tr.TLSClientConfig.InsecureSkipVerify {
		t.Error("expected InsecureSkipVerify=false")
	}
}

func TestBuildClient_MTLS_ValidCertKey(t *testing.T) {
	dir := t.TempDir()
	certFile, keyFile, _ := generateSelfSignedCert(t, dir)

	client, err := BuildClient(ClientConfig{
		TimeoutSeconds: 5,
		CertFile:       certFile,
		KeyFile:        keyFile,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	tr := client.Transport.(*http.Transport)
	if len(tr.TLSClientConfig.Certificates) != 1 {
		t.Errorf("expected 1 client certificate, got %d", len(tr.TLSClientConfig.Certificates))
	}
}

func TestBuildClient_MTLS_OnlyCertFile_Errors(t *testing.T) {
	dir := t.TempDir()
	certFile, _, _ := generateSelfSignedCert(t, dir)

	_, err := BuildClient(ClientConfig{CertFile: certFile, KeyFile: ""})
	if err == nil {
		t.Error("expected error when only certFile is set without keyFile")
	}
}

func TestBuildClient_MTLS_OnlyKeyFile_Errors(t *testing.T) {
	dir := t.TempDir()
	_, keyFile, _ := generateSelfSignedCert(t, dir)

	_, err := BuildClient(ClientConfig{CertFile: "", KeyFile: keyFile})
	if err == nil {
		t.Error("expected error when only keyFile is set without certFile")
	}
}

func TestBuildClient_MTLS_MissingFiles_Errors(t *testing.T) {
	_, err := BuildClient(ClientConfig{
		CertFile: "/nonexistent/client.crt",
		KeyFile:  "/nonexistent/client.key",
	})
	if err == nil {
		t.Error("expected error for missing cert/key files")
	}
}

func TestBuildClient_CustomCA(t *testing.T) {
	dir := t.TempDir()
	caFile := filepath.Join(dir, "ca.crt")
	// Use the self-signed cert as the CA
	_, _, certPEM := generateSelfSignedCert(t, dir)
	if err := os.WriteFile(caFile, certPEM, 0644); err != nil {
		t.Fatalf("write CA: %v", err)
	}

	client, err := BuildClient(ClientConfig{
		TimeoutSeconds: 5,
		CACertFile:     caFile,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	tr := client.Transport.(*http.Transport)
	if tr.TLSClientConfig.RootCAs == nil {
		t.Error("expected non-nil RootCAs pool")
	}
}

func TestBuildClient_CustomCA_MissingFile_Errors(t *testing.T) {
	_, err := BuildClient(ClientConfig{CACertFile: "/nonexistent/ca.crt"})
	if err == nil {
		t.Error("expected error for missing CA cert file")
	}
}

func TestBuildClient_CustomCA_InvalidPEM_Errors(t *testing.T) {
	dir := t.TempDir()
	caFile := filepath.Join(dir, "bad-ca.crt")
	if err := os.WriteFile(caFile, []byte("not valid PEM content"), 0644); err != nil {
		t.Fatalf("write bad CA: %v", err)
	}

	_, err := BuildClient(ClientConfig{CACertFile: caFile})
	if err == nil {
		t.Error("expected error for non-PEM CA file")
	}
}

func TestBuildClient_MTLSAndCustomCA(t *testing.T) {
	dir := t.TempDir()
	certFile, keyFile, certPEM := generateSelfSignedCert(t, dir)

	// Use the same self-signed cert as the CA
	caFile := filepath.Join(dir, "ca.crt")
	if err := os.WriteFile(caFile, certPEM, 0644); err != nil {
		t.Fatalf("write CA: %v", err)
	}

	client, err := BuildClient(ClientConfig{
		TimeoutSeconds: 10,
		CertFile:       certFile,
		KeyFile:        keyFile,
		CACertFile:     caFile,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	tr := client.Transport.(*http.Transport)
	if len(tr.TLSClientConfig.Certificates) != 1 {
		t.Errorf("expected 1 client certificate, got %d", len(tr.TLSClientConfig.Certificates))
	}
	if tr.TLSClientConfig.RootCAs == nil {
		t.Error("expected non-nil RootCAs pool")
	}
}

// TestBuildClient_MTLS_FunctionalRoundTrip verifies the client can actually
// connect to an mTLS server using the generated certificate.
func TestBuildClient_MTLS_FunctionalRoundTrip(t *testing.T) {
	dir := t.TempDir()
	certFile, keyFile, certPEM := generateSelfSignedCert(t, dir)

	// Build a test TLS server that requires a client certificate from our pool
	pool := x509.NewCertPool()
	pool.AppendCertsFromPEM(certPEM)

	serverCert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		t.Fatalf("load server cert: %v", err)
	}

	tlsServer := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	}))
	tlsServer.TLS = &tls.Config{
		Certificates: []tls.Certificate{serverCert},
		ClientCAs:    pool,
		ClientAuth:   tls.RequireAndVerifyClientCert,
	}
	tlsServer.StartTLS()
	defer tlsServer.Close()

	client, err := BuildClient(ClientConfig{
		TimeoutSeconds:     5,
		InsecureSkipVerify: true, // self-signed server cert
		CertFile:           certFile,
		KeyFile:            keyFile,
	})
	if err != nil {
		t.Fatalf("BuildClient error: %v", err)
	}

	resp, err := client.Get(tlsServer.URL)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

// ---- SetHTTPClient (Recorder) ----

func TestSetHTTPClient_ReplacesClient(t *testing.T) {
	rec, _ := makeTestRecorder(t)
	original := rec.httpClient

	custom := &http.Client{Timeout: 99 * time.Second}
	rec.SetHTTPClient(custom)

	if rec.httpClient == original {
		t.Error("expected httpClient to be replaced")
	}
	if rec.httpClient.Timeout != 99*time.Second {
		t.Errorf("expected timeout 99s, got %v", rec.httpClient.Timeout)
	}
}

func TestSetHTTPClient_UsedForRequests(t *testing.T) {
	backend := startFakeBackend(202, `{"set":"true"}`, nil)
	defer backend.Close()

	rec, _ := makeTestRecorder(t)

	// Build a custom client and set it on the recorder
	custom, err := BuildClient(ClientConfig{TimeoutSeconds: 10, InsecureSkipVerify: true})
	if err != nil {
		t.Fatalf("BuildClient: %v", err)
	}
	rec.SetHTTPClient(custom)

	spec := &models.Spec{BackendURI: backend.URL}
	op := &models.Operation{ID: "op-custom"}

	status, _, body, err := rec.ProxyAndRecord("GET", "/x", "", http.Header{}, "", op, spec, "sig-custom")
	if err != nil {
		t.Fatalf("ProxyAndRecord: %v", err)
	}
	if status != 202 {
		t.Errorf("expected 202, got %d", status)
	}
	if body != `{"set":"true"}` {
		t.Errorf("unexpected body: %q", body)
	}
}

// ---- SetProxyHTTPClient (Engine) ----

func TestSetProxyHTTPClient_Delegates(t *testing.T) {
	engine, _ := setupTestEngine(t)

	custom := &http.Client{Timeout: 77 * time.Second}
	engine.SetProxyHTTPClient(custom)

	if engine.recorder.httpClient.Timeout != 77*time.Second {
		t.Errorf("expected recorder timeout 77s, got %v", engine.recorder.httpClient.Timeout)
	}
}
