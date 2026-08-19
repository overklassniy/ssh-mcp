package ssh

import (
	"bufio"
	"context"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"log/slog"
	"net"
	"net/url"
	"strings"
	"time"

	"golang.org/x/net/proxy"
)

// dialProxy dials the target address through the given proxy URL.
// Supports SOCKS5 (socks5://), HTTP CONNECT (http://), and HTTPS CONNECT (https://).
// Returns a net.Conn connected to the target, or an error.
func dialProxy(ctx context.Context, proxyURL, target string) (net.Conn, error) {
	u, err := url.Parse(proxyURL)
	if err != nil {
		return nil, fmt.Errorf("parse proxy URL: %w", err)
	}

	scheme := strings.ToLower(u.Scheme)
	logProxyDial(proxyURL, target)

	switch scheme {
	case "socks5", "socks5h":
		return dialSOCKS5(ctx, u, target)
	case "http", "https":
		return dialHTTPConnect(ctx, u, target, scheme == "https")
	default:
		return nil, fmt.Errorf("unsupported proxy scheme: %s", scheme)
	}
}

// dialSOCKS5 dials through a SOCKS5 proxy using golang.org/x/net/proxy.
func dialSOCKS5(ctx context.Context, u *url.URL, target string) (net.Conn, error) {
	var auth *proxy.Auth
	if u.User != nil {
		user := u.User.Username()
		pass, _ := u.User.Password()
		if user != "" {
			auth = &proxy.Auth{User: user, Password: pass}
		}
	}

	dialer, err := proxy.SOCKS5("tcp", u.Host, auth, &net.Dialer{
		Timeout: 30 * time.Second,
	})
	if err != nil {
		return nil, fmt.Errorf("create SOCKS5 dialer: %w", err)
	}

	type result struct {
		conn net.Conn
		err  error
	}
	ch := make(chan result, 1)
	go func() {
		c, e := dialer.Dial("tcp", target)
		ch <- result{c, e}
	}()

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case r := <-ch:
		if r.err != nil {
			return nil, fmt.Errorf("SOCKS5 dial: %w", r.err)
		}
		return r.conn, nil
	}
}

// dialHTTPConnect dials through an HTTP or HTTPS CONNECT proxy.
// It establishes a TCP/TLS connection to the proxy, sends a CONNECT request,
// and returns the raw connection after a successful 200 response.
func dialHTTPConnect(ctx context.Context, u *url.URL, target string, useTLS bool) (net.Conn, error) {
	proxyHost := u.Host
	if !strings.Contains(proxyHost, ":") {
		if useTLS {
			proxyHost += ":443"
		} else {
			proxyHost += ":80"
		}
	}

	dialer := &net.Dialer{Timeout: 30 * time.Second}
	var conn net.Conn
	var err error

	if useTLS {
		rawConn, derr := dialer.DialContext(ctx, "tcp", proxyHost)
		if derr != nil {
			return nil, fmt.Errorf("connect to proxy %s: %w", proxyHost, derr)
		}
		tlsConn := tls.Client(rawConn, &tls.Config{ServerName: hostOnly(u.Host)})
		if herr := tlsConn.Handshake(); herr != nil {
			rawConn.Close()
			return nil, fmt.Errorf("TLS handshake with proxy: %w", herr)
		}
		conn = tlsConn
	} else {
		conn, err = dialer.DialContext(ctx, "tcp", proxyHost)
		if err != nil {
			return nil, fmt.Errorf("connect to proxy %s: %w", proxyHost, err)
		}
	}

	// Build CONNECT request
	var authHeader string
	if u.User != nil {
		user := u.User.Username()
		pass, _ := u.User.Password()
		if user != "" {
			creds := user + ":" + pass
			authHeader = "Proxy-Authorization: Basic " + base64.StdEncoding.EncodeToString([]byte(creds)) + "\r\n"
		}
	}

	request := fmt.Sprintf("CONNECT %s HTTP/1.1\r\nHost: %s\r\n%sUser-Agent: ssh-mcp\r\n\r\n",
		target, target, authHeader)

	if _, err := conn.Write([]byte(request)); err != nil {
		conn.Close()
		return nil, fmt.Errorf("send CONNECT request: %w", err)
	}

	// Read response
	br := bufio.NewReader(conn)
	respLine, err := br.ReadString('\n')
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("read CONNECT response: %w", err)
	}

	if !strings.Contains(respLine, "200") {
		conn.Close()
		return nil, fmt.Errorf("proxy CONNECT failed: %s", strings.TrimSpace(respLine))
	}

	// Read and discard remaining headers until empty line
	for {
		line, err := br.ReadString('\n')
		if err != nil {
			conn.Close()
			return nil, fmt.Errorf("read CONNECT headers: %w", err)
		}
		if line == "\r\n" || line == "\n" {
			break
		}
	}

	// If the bufio.Reader has buffered data, it may contain initial bytes
	// from the target. We need to return a connection that includes them.
	if br.Buffered() > 0 {
		buffered, _ := br.Peek(br.Buffered())
		buf := make([]byte, len(buffered))
		copy(buf, buffered)
		return &bufferedConn{Conn: conn, buf: buf}, nil
	}

	return conn, nil
}

// hostOnly extracts the hostname from a host:port string.
func hostOnly(hostPort string) string {
	host, _, err := net.SplitHostPort(hostPort)
	if err != nil {
		return hostPort
	}
	return host
}

// bufferedConn wraps a net.Conn with pre-read buffered bytes.
type bufferedConn struct {
	net.Conn
	buf      []byte
	consumed bool
}

func (bc *bufferedConn) Read(p []byte) (int, error) {
	if !bc.consumed && len(bc.buf) > 0 {
		n := copy(p, bc.buf)
		bc.buf = bc.buf[n:]
		if len(bc.buf) == 0 {
			bc.consumed = true
		}
		return n, nil
	}
	return bc.Conn.Read(p)
}

// redactProxyURL removes credentials from a proxy URL for safe logging.
func redactProxyURL(proxyURL string) string {
	u, err := url.Parse(proxyURL)
	if err != nil {
		return "[invalid proxy URL]"
	}
	if u.User != nil {
		u.User = url.User(u.User.Username())
	}
	return u.String()
}

// logProxyDial logs a proxy dial attempt with redacted credentials.
func logProxyDial(proxyURL, target string) {
	slog.Debug("dialing through proxy",
		"proxy", redactProxyURL(proxyURL),
		"target", target)
}
