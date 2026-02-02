package tlsutil

import (
	"crypto/tls"
	"io"
	"net"
	"sync"
	"time"
)

// MuxListener is a listener that can detect TLS connections and route them appropriately
type MuxListener struct {
	inner     net.Listener
	tlsConfig *tls.Config

	httpConns  chan net.Conn
	httpsConns chan net.Conn

	closeOnce sync.Once
	closed    chan struct{}
}

// NewMuxListener creates a listener that multiplexes HTTP and HTTPS on the same port
func NewMuxListener(inner net.Listener, tlsConfig *tls.Config) *MuxListener {
	ml := &MuxListener{
		inner:      inner,
		tlsConfig:  tlsConfig,
		httpConns:  make(chan net.Conn, 128),
		httpsConns: make(chan net.Conn, 128),
		closed:     make(chan struct{}),
	}

	go ml.serve()

	return ml
}

// serve accepts connections and routes them based on TLS detection
func (ml *MuxListener) serve() {
	for {
		conn, err := ml.inner.Accept()
		if err != nil {
			select {
			case <-ml.closed:
				return
			default:
				continue
			}
		}

		go ml.handleConn(conn)
	}
}

// handleConn detects if the connection is TLS and routes it appropriately
func (ml *MuxListener) handleConn(conn net.Conn) {
	// Peek at the first byte to detect TLS
	peekedConn := &peekedConn{Conn: conn}

	// Set a short deadline for the peek
	conn.SetReadDeadline(time.Now().Add(5 * time.Second))

	buf := make([]byte, 1)
	n, err := conn.Read(buf)

	// Reset deadline
	conn.SetReadDeadline(time.Time{})

	if err != nil || n == 0 {
		conn.Close()
		return
	}

	peekedConn.peeked = buf[:n]

	// TLS handshake starts with 0x16 (TLS record type for handshake)
	isTLS := buf[0] == 0x16

	select {
	case <-ml.closed:
		peekedConn.Close()
		return
	default:
	}

	if isTLS {
		// Wrap with TLS
		tlsConn := tls.Server(peekedConn, ml.tlsConfig)
		select {
		case ml.httpsConns <- tlsConn:
		case <-ml.closed:
			tlsConn.Close()
		}
	} else {
		select {
		case ml.httpConns <- peekedConn:
		case <-ml.closed:
			peekedConn.Close()
		}
	}
}

// HTTPListener returns a listener for HTTP connections
func (ml *MuxListener) HTTPListener() net.Listener {
	return &chanListener{
		conns:  ml.httpConns,
		closed: ml.closed,
		addr:   ml.inner.Addr(),
	}
}

// HTTPSListener returns a listener for HTTPS connections
func (ml *MuxListener) HTTPSListener() net.Listener {
	return &chanListener{
		conns:  ml.httpsConns,
		closed: ml.closed,
		addr:   ml.inner.Addr(),
	}
}

// Close closes the multiplexed listener
func (ml *MuxListener) Close() error {
	ml.closeOnce.Do(func() {
		close(ml.closed)
	})
	return ml.inner.Close()
}

// Addr returns the listener's network address
func (ml *MuxListener) Addr() net.Addr {
	return ml.inner.Addr()
}

// peekedConn is a connection that has had some bytes peeked
type peekedConn struct {
	net.Conn
	peeked []byte
}

func (c *peekedConn) Read(b []byte) (int, error) {
	if len(c.peeked) > 0 {
		n := copy(b, c.peeked)
		c.peeked = c.peeked[n:]
		return n, nil
	}
	return c.Conn.Read(b)
}

// chanListener implements net.Listener using a channel
type chanListener struct {
	conns  chan net.Conn
	closed chan struct{}
	addr   net.Addr
}

func (cl *chanListener) Accept() (net.Conn, error) {
	select {
	case conn := <-cl.conns:
		return conn, nil
	case <-cl.closed:
		return nil, io.EOF
	}
}

func (cl *chanListener) Close() error {
	return nil // Close is handled by MuxListener
}

func (cl *chanListener) Addr() net.Addr {
	return cl.addr
}
