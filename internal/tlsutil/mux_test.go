package tlsutil

import (
	"crypto/tls"
	"net"
	"testing"
	"time"
)

func TestNewMuxListener(t *testing.T) {
	// Create a test listener
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to create listener: %v", err)
	}
	defer listener.Close()

	tlsConfig := &tls.Config{
		MinVersion: tls.VersionTLS12,
	}

	mux := NewMuxListener(listener, tlsConfig)
	if mux == nil {
		t.Fatal("NewMuxListener returned nil")
	}
	defer mux.Close()

	if mux.Addr() != listener.Addr() {
		t.Error("MuxListener address should match inner listener address")
	}
}

func TestMuxListener_HTTPListener(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to create listener: %v", err)
	}

	tlsConfig := &tls.Config{
		MinVersion: tls.VersionTLS12,
	}

	mux := NewMuxListener(listener, tlsConfig)
	defer mux.Close()

	httpListener := mux.HTTPListener()
	if httpListener == nil {
		t.Fatal("HTTPListener returned nil")
	}

	if httpListener.Addr() != listener.Addr() {
		t.Error("HTTP listener address should match")
	}
}

func TestMuxListener_HTTPSListener(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to create listener: %v", err)
	}

	tlsConfig := &tls.Config{
		MinVersion: tls.VersionTLS12,
	}

	mux := NewMuxListener(listener, tlsConfig)
	defer mux.Close()

	httpsListener := mux.HTTPSListener()
	if httpsListener == nil {
		t.Fatal("HTTPSListener returned nil")
	}

	if httpsListener.Addr() != listener.Addr() {
		t.Error("HTTPS listener address should match")
	}
}

func TestMuxListener_DetectHTTP(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to create listener: %v", err)
	}

	tlsConfig := &tls.Config{
		MinVersion: tls.VersionTLS12,
	}

	mux := NewMuxListener(listener, tlsConfig)
	defer mux.Close()

	// Connect and send HTTP request (starts with "G" for GET)
	go func() {
		conn, err := net.Dial("tcp", listener.Addr().String())
		if err != nil {
			t.Errorf("Failed to connect: %v", err)
			return
		}
		defer conn.Close()

		// Send HTTP GET request start
		conn.Write([]byte("GET / HTTP/1.1\r\n"))
	}()

	httpListener := mux.HTTPListener()

	// Set timeout for Accept
	done := make(chan struct{})
	go func() {
		conn, err := httpListener.Accept()
		if err != nil {
			return
		}
		conn.Close()
		close(done)
	}()

	select {
	case <-done:
		// Success - HTTP connection was detected
	case <-time.After(2 * time.Second):
		t.Error("Timeout waiting for HTTP connection")
	}
}

func TestPeekedConn_Read(t *testing.T) {
	// Create a pipe for testing
	server, client := net.Pipe()
	defer server.Close()
	defer client.Close()

	// Write some data
	go func() {
		client.Write([]byte("hello world"))
	}()

	// Create peeked conn with some pre-read data
	peeked := &peekedConn{
		Conn:   server,
		peeked: []byte("pre"),
	}

	// First read should return peeked data
	buf := make([]byte, 10)
	n, err := peeked.Read(buf)
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if string(buf[:n]) != "pre" {
		t.Errorf("Expected 'pre', got %q", string(buf[:n]))
	}

	// Second read should return actual data
	n, err = peeked.Read(buf)
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if string(buf[:n]) != "hello worl" {
		t.Errorf("Expected 'hello worl', got %q", string(buf[:n]))
	}
}

func TestChanListener_Close(t *testing.T) {
	cl := &chanListener{
		conns:  make(chan net.Conn, 1),
		closed: make(chan struct{}),
		addr:   &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 8080},
	}

	// Close should not error
	if err := cl.Close(); err != nil {
		t.Errorf("Close should not return error, got: %v", err)
	}
}

func TestChanListener_Addr(t *testing.T) {
	addr := &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 8080}
	cl := &chanListener{
		conns:  make(chan net.Conn, 1),
		closed: make(chan struct{}),
		addr:   addr,
	}

	if cl.Addr() != addr {
		t.Error("Addr should return the stored address")
	}
}

func TestMuxListener_DetectTLS(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to create listener: %v", err)
	}

	// Minimal self-signed TLS config so handleConn can wrap the connection
	cm := NewCertificateManager("", "", t.TempDir())
	cert, err := cm.GetCertificate(true)
	if err != nil {
		t.Fatalf("Failed to generate cert: %v", err)
	}

	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{*cert},
		MinVersion:   tls.VersionTLS12,
	}

	mux := NewMuxListener(listener, tlsConfig)
	defer mux.Close()

	// Connect and send a byte that looks like a TLS ClientHello (0x16)
	go func() {
		conn, err := net.Dial("tcp", listener.Addr().String())
		if err != nil {
			return
		}
		defer conn.Close()
		conn.Write([]byte{0x16}) // TLS record type for handshake
		// Keep conn open briefly so handleConn can route it
		time.Sleep(100 * time.Millisecond)
	}()

	httpsListener := mux.HTTPSListener()

	done := make(chan struct{})
	go func() {
		conn, err := httpsListener.Accept()
		if err != nil {
			return
		}
		conn.Close()
		close(done)
	}()

	select {
	case <-done:
		// Success – TLS connection was correctly detected and routed
	case <-time.After(3 * time.Second):
		t.Error("Timeout waiting for TLS connection on HTTPS listener")
	}
}

func TestMuxListener_HandleConn_ReadError(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to create listener: %v", err)
	}

	tlsConfig := &tls.Config{MinVersion: tls.VersionTLS12}
	mux := NewMuxListener(listener, tlsConfig)
	defer mux.Close()

	// Connect then immediately close without sending any data;
	// handleConn should receive a read error and close the connection without panicking.
	conn, err := net.Dial("tcp", listener.Addr().String())
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	conn.Close() // triggers read error inside handleConn

	// Give handleConn time to process the error
	time.Sleep(200 * time.Millisecond)
	// If we reach here without panicking, the error path is handled correctly.
}

func TestChanListener_Accept_ClosedChannel(t *testing.T) {
	conns := make(chan net.Conn)
	closed := make(chan struct{})

	cl := &chanListener{
		conns:  conns,
		closed: closed,
		addr:   &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 8080},
	}

	// Close the channel and verify Accept returns ErrClosed
	close(closed)

	conn, err := cl.Accept()
	if err == nil {
		t.Error("Expected error from Accept when closed")
		if conn != nil {
			conn.Close()
		}
	}
}

func TestChanListener_Accept_ChannelClosed(t *testing.T) {
	conns := make(chan net.Conn)
	closed := make(chan struct{})

	cl := &chanListener{
		conns:  conns,
		closed: closed,
		addr:   &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 8080},
	}

	// Close the conns channel (ok = false path)
	close(conns)

	conn, err := cl.Accept()
	if err == nil {
		t.Error("Expected error from Accept when conns channel is closed")
		if conn != nil {
			conn.Close()
		}
	}
}
