package main

import (
	"bufio"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"
)

const tlsHandshakeRecordType = 0x16

type bufferedConn struct {
	net.Conn
	reader *bufio.Reader
}

func newBufferedConn(conn net.Conn) *bufferedConn {
	return &bufferedConn{
		Conn:   conn,
		reader: bufio.NewReader(conn),
	}
}

func (c *bufferedConn) Read(p []byte) (int, error) {
	return c.reader.Read(p)
}

type protocolListener struct {
	addr      net.Addr
	conns     chan net.Conn
	done      chan struct{}
	closeOnce sync.Once
}

func newProtocolListener(addr net.Addr) *protocolListener {
	return &protocolListener{
		addr:  addr,
		conns: make(chan net.Conn),
		done:  make(chan struct{}),
	}
}

func (l *protocolListener) Accept() (net.Conn, error) {
	select {
	case conn, ok := <-l.conns:
		if !ok {
			return nil, net.ErrClosed
		}
		return conn, nil
	case <-l.done:
		return nil, net.ErrClosed
	}
}

func (l *protocolListener) Close() error {
	l.closeOnce.Do(func() {
		close(l.done)
	})
	return nil
}

func (l *protocolListener) Addr() net.Addr {
	return l.addr
}

func (l *protocolListener) deliver(conn net.Conn) bool {
	select {
	case <-l.done:
		return false
	case l.conns <- conn:
		return true
	}
}

func (l *protocolListener) finish() {
	close(l.conns)
}

func serveHTTPAndTLSOnSamePort(ctx context.Context, server *http.Server, certFile, keyFile string, shutdownTimeout time.Duration) error {
	baseListener, err := net.Listen("tcp", server.Addr)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", server.Addr, err)
	}

	httpServer := *server
	tlsServer := *server

	tlsConfig := &tls.Config{MinVersion: tls.VersionTLS12}
	if server.TLSConfig != nil {
		tlsConfig = server.TLSConfig.Clone()
	}

	certificate, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return fmt.Errorf("load tls certificate: %w", err)
	}
	tlsConfig.Certificates = []tls.Certificate{certificate}
	tlsServer.TLSConfig = tlsConfig

	httpListener := newProtocolListener(baseListener.Addr())
	tlsListener := newProtocolListener(baseListener.Addr())

	var shutdownErr error
	var shutdownMu sync.Mutex
	var shutdownOnce sync.Once

	shutdownAll := func() {
		shutdownOnce.Do(func() {
			if err := baseListener.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
				shutdownMu.Lock()
				shutdownErr = errors.Join(shutdownErr, err)
				shutdownMu.Unlock()
			}

			shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
			defer cancel()

			if err := httpServer.Shutdown(shutdownCtx); err != nil && !errors.Is(err, http.ErrServerClosed) {
				shutdownMu.Lock()
				shutdownErr = errors.Join(shutdownErr, err)
				shutdownMu.Unlock()
			}
			if err := tlsServer.Shutdown(shutdownCtx); err != nil && !errors.Is(err, http.ErrServerClosed) {
				shutdownMu.Lock()
				shutdownErr = errors.Join(shutdownErr, err)
				shutdownMu.Unlock()
			}
		})
	}

	errCh := make(chan error, 3)

	go func() {
		<-ctx.Done()
		shutdownAll()
	}()

	go func() {
		errCh <- dispatchProtocolConnections(baseListener, httpListener, tlsListener)
	}()
	go func() {
		errCh <- normalizeServeError(httpServer.Serve(httpListener))
	}()
	go func() {
		errCh <- normalizeServeError(tlsServer.Serve(tls.NewListener(tlsListener, tlsConfig)))
	}()

	var serveErr error
	for remaining := 3; remaining > 0; remaining-- {
		if err := <-errCh; err != nil {
			serveErr = errors.Join(serveErr, err)
			shutdownAll()
		}
	}

	shutdownMu.Lock()
	defer shutdownMu.Unlock()
	return errors.Join(serveErr, shutdownErr)
}

func dispatchProtocolConnections(baseListener net.Listener, httpListener, tlsListener *protocolListener) error {
	defer httpListener.finish()
	defer tlsListener.finish()

	for {
		conn, err := baseListener.Accept()
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				return nil
			}
			return err
		}

		buffered := newBufferedConn(conn)
		header, err := buffered.reader.Peek(1)
		if err != nil {
			_ = conn.Close()
			continue
		}

		target := httpListener
		if len(header) == 1 && header[0] == tlsHandshakeRecordType {
			target = tlsListener
		}

		if !target.deliver(buffered) {
			_ = conn.Close()
		}
	}
}

func normalizeServeError(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, http.ErrServerClosed):
		return nil
	case errors.Is(err, net.ErrClosed):
		return nil
	default:
		return err
	}
}
