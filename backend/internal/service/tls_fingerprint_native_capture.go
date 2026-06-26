package service

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"time"
)

func (l *tlsFingerprintNativeCaptureNetListener) Accept() (net.Conn, error) {
	conn, err := l.Listener.Accept()
	if err != nil {
		return nil, err
	}
	return &tlsFingerprintNativeCaptureConn{Conn: conn, onClose: l.onClose}, nil
}

func (c *tlsFingerprintNativeCaptureConn) Read(p []byte) (int, error) {
	n, err := c.Conn.Read(p)
	if n > 0 {
		c.capture(p[:n])
	}
	return n, err
}

func (c *tlsFingerprintNativeCaptureConn) RawClientHello() []byte {
	if c == nil {
		return nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]byte(nil), c.record...)
}

func (c *tlsFingerprintNativeCaptureConn) Close() error {
	if c != nil && c.onClose != nil {
		c.onClose(c)
	}
	return c.Conn.Close()
}

func (c *tlsFingerprintNativeCaptureConn) Captured() bool {
	if c == nil {
		return false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.record) > 0
}

func (c *tlsFingerprintNativeCaptureConn) capture(chunk []byte) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.done || len(chunk) == 0 {
		return
	}
	for len(chunk) > 0 {
		if len(c.record) == 0 && chunk[0] != 22 {
			c.done = true
			return
		}
		need := tlsFingerprintNativeCaptureRecordLength(c.record)
		if need == 0 {
			limit := len(chunk)
			if limit > 5-len(c.record) {
				limit = 5 - len(c.record)
			}
			c.record = append(c.record, chunk[:limit]...)
			chunk = chunk[limit:]
			continue
		}
		remaining := need - len(c.record)
		if remaining <= 0 {
			c.done = true
			return
		}
		if remaining > len(chunk) {
			remaining = len(chunk)
		}
		c.record = append(c.record, chunk[:remaining]...)
		chunk = chunk[remaining:]
		if len(c.record) >= need {
			c.done = true
			return
		}
	}
}

func tlsFingerprintNativeCaptureRecordLength(record []byte) int {
	if len(record) < 5 {
		return 0
	}
	return 5 + (int(record[3]) << 8) + int(record[4])
}

func generateTLSFingerprintNativeCaptureSelfSignedCert() (tls.Certificate, error) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("generate TLS capture key: %w", err)
	}
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("generate TLS capture serial: %w", err)
	}
	template := x509.Certificate{
		SerialNumber:          serial,
		Subject:               pkix.Name{CommonName: "sub2api tls fingerprint capture"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		DNSNames:              []string{"localhost"},
		IPAddresses:           []net.IP{net.ParseIP("127.0.0.1"), net.ParseIP("::1")},
	}
	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &key.PublicKey, key)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("generate TLS capture certificate: %w", err)
	}
	keyDER := x509.MarshalPKCS1PrivateKey(key)
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: keyDER})
	cert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("load generated TLS capture certificate: %w", err)
	}
	return cert, nil
}
