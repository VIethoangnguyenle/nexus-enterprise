package testutil

import (
	"bufio"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"io"
	"math/big"
	"net"
	"strings"
	"sync"
	"testing"
	"time"
)

// FakeSMTP is a minimal in-process SMTP server: enough of the dialogue for
// net/smtp (EHLO, STARTTLS, AUTH PLAIN, MAIL, RCPT, DATA, QUIT). It listens on
// loopback with a self-signed certificate, trusted through Pool.
type FakeSMTP struct {
	Host string
	Port string
	// Pool trusts the server's certificate: pass it as RootCAs.
	Pool     *x509.CertPool
	implicit bool

	// Behaviour switches, set (through the mutate argument) before the first connection.
	Hang       bool // accept, send nothing
	RejectData bool // 554 after DATA
	NoSTARTTLS bool

	mu       sync.Mutex
	conns    int
	auths    []string // decoded AUTH PLAIN payloads
	froms    []string
	rcpts    []string
	messages []string
}

// NewFakeSMTP starts a fake server for the life of the test. implicit serves TLS
// from the first byte (port 465 style); otherwise it offers STARTTLS.
func NewFakeSMTP(t *testing.T, implicit bool, mutate func(*FakeSMTP)) *FakeSMTP {
	t.Helper()
	cert, pool := selfSignedCert(t)
	f := &FakeSMTP{Pool: pool, implicit: implicit}
	if mutate != nil {
		mutate(f)
	}
	tlsCfg := &tls.Config{Certificates: []tls.Certificate{cert}, MinVersion: tls.VersionTLS12}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	if implicit {
		ln = tls.NewListener(ln, tlsCfg)
	}
	f.Host, f.Port, _ = net.SplitHostPort(ln.Addr().String())
	t.Cleanup(func() { ln.Close() })
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			go f.serve(c, tlsCfg)
		}
	}()
	return f
}

func selfSignedCert(t *testing.T) (tls.Certificate, *x509.CertPool) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "fake smtp"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1")},
	}
	der, err := x509.CreateCertificate(rand.Reader, tpl, tpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	leaf, _ := x509.ParseCertificate(der)
	pool := x509.NewCertPool()
	pool.AddCert(leaf)
	return tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key}, pool
}

func (f *FakeSMTP) record(dst *[]string, v string) {
	f.mu.Lock()
	*dst = append(*dst, v)
	f.mu.Unlock()
}

func (f *FakeSMTP) serve(c net.Conn, tlsCfg *tls.Config) {
	f.mu.Lock()
	f.conns++
	f.mu.Unlock()
	defer func() { c.Close() }()
	if f.Hang {
		time.Sleep(5 * time.Second)
		return
	}
	_ = c.SetDeadline(time.Now().Add(10 * time.Second))
	r := bufio.NewReader(c)
	say := func(s string) { _, _ = io.WriteString(c, s+"\r\n") }
	secure := f.implicit
	say("220 fake ESMTP")
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return
		}
		line = strings.TrimRight(line, "\r\n")
		up := strings.ToUpper(line)
		switch {
		case strings.HasPrefix(up, "EHLO"), strings.HasPrefix(up, "HELO"):
			say("250-fake")
			if !secure && !f.NoSTARTTLS {
				say("250-STARTTLS")
			}
			say("250-AUTH PLAIN")
			say("250 8BITMIME")
		case up == "STARTTLS":
			say("220 go ahead")
			tc := tls.Server(c, tlsCfg)
			if err := tc.Handshake(); err != nil {
				return
			}
			c = tc
			r = bufio.NewReader(tc)
			secure = true
		case strings.HasPrefix(up, "AUTH PLAIN"):
			raw, _ := base64.StdEncoding.DecodeString(strings.TrimSpace(line[len("AUTH PLAIN"):]))
			f.record(&f.auths, string(raw))
			say("235 ok")
		case strings.HasPrefix(up, "MAIL FROM:"):
			f.record(&f.froms, line[len("MAIL FROM:"):])
			say("250 ok")
		case strings.HasPrefix(up, "RCPT TO:"):
			f.record(&f.rcpts, line[len("RCPT TO:"):])
			say("250 ok")
		case up == "DATA":
			say("354 go")
			var sb strings.Builder
			for {
				l, err := r.ReadString('\n')
				if err != nil {
					return
				}
				if l == ".\r\n" {
					break
				}
				sb.WriteString(strings.TrimPrefix(l, "."))
			}
			if f.RejectData {
				say("554 rejected")
				continue
			}
			f.record(&f.messages, sb.String())
			say("250 queued")
		case up == "QUIT":
			say("221 bye")
			return
		default:
			say("502 unsupported")
		}
	}
}

// Conns is how many connections were opened.
func (f *FakeSMTP) Conns() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.conns
}

// Messages are the raw messages accepted, in order.
func (f *FakeSMTP) Messages() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.messages...)
}

// Auths are the decoded AUTH PLAIN payloads received.
func (f *FakeSMTP) Auths() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.auths...)
}

// Froms are the MAIL FROM arguments received.
func (f *FakeSMTP) Froms() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.froms...)
}

// Rcpts are the RCPT TO arguments received.
func (f *FakeSMTP) Rcpts() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.rcpts...)
}
