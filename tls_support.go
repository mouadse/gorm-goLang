package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"os"
	"strings"
	"time"
)

const (
	defaultTLSCertPathEnv = "TLS_CERT_FILE"
	defaultTLSKeyPathEnv  = "TLS_KEY_FILE"
	defaultTLSHostsEnv    = "TLS_SELF_SIGNED_HOSTS"
)

func resolveTLSCertificateFiles() (string, string, error) {
	certFile := strings.TrimSpace(os.Getenv(defaultTLSCertPathEnv))
	keyFile := strings.TrimSpace(os.Getenv(defaultTLSKeyPathEnv))

	switch {
	case certFile != "" && keyFile != "":
		return certFile, keyFile, nil
	case certFile != "" || keyFile != "":
		return "", "", fmt.Errorf("%s and %s must be set together", defaultTLSCertPathEnv, defaultTLSKeyPathEnv)
	default:
		return generateSelfSignedCertificateFiles()
	}
}

func generateSelfSignedCertificateFiles() (string, string, error) {
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return "", "", fmt.Errorf("generate private key: %w", err)
	}

	serialLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialLimit)
	if err != nil {
		return "", "", fmt.Errorf("generate serial number: %w", err)
	}

	hostnames, ipAddresses := resolveSelfSignedHosts()
	certificateTemplate := &x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			CommonName: "fitness-tracker.local",
		},
		NotBefore:             time.Now().UTC().Add(-5 * time.Minute),
		NotAfter:              time.Now().UTC().Add(365 * 24 * time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		DNSNames:              hostnames,
		IPAddresses:           ipAddresses,
	}

	certificateDER, err := x509.CreateCertificate(rand.Reader, certificateTemplate, certificateTemplate, &privateKey.PublicKey, privateKey)
	if err != nil {
		return "", "", fmt.Errorf("create certificate: %w", err)
	}

	keyDER, err := x509.MarshalECPrivateKey(privateKey)
	if err != nil {
		return "", "", fmt.Errorf("marshal private key: %w", err)
	}

	tempDir, err := os.MkdirTemp("", "fitness-tracker-tls-*")
	if err != nil {
		return "", "", fmt.Errorf("create tls temp dir: %w", err)
	}

	certFile := tempDir + "/cert.pem"
	keyFile := tempDir + "/key.pem"
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certificateDER})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})

	if err := os.WriteFile(certFile, certPEM, 0o600); err != nil {
		return "", "", fmt.Errorf("write certificate: %w", err)
	}
	if err := os.WriteFile(keyFile, keyPEM, 0o600); err != nil {
		return "", "", fmt.Errorf("write private key: %w", err)
	}

	return certFile, keyFile, nil
}

func resolveSelfSignedHosts() ([]string, []net.IP) {
	defaultHosts := []string{"localhost", "app", "fitness-app"}
	if hostname, err := os.Hostname(); err == nil {
		hostname = strings.TrimSpace(hostname)
		if hostname != "" {
			defaultHosts = append(defaultHosts, hostname)
		}
	}

	configuredHosts := splitCommaSeparatedValues(os.Getenv(defaultTLSHostsEnv))
	if len(configuredHosts) == 0 {
		configuredHosts = append(configuredHosts, defaultHosts...)
		configuredHosts = append(configuredHosts, "127.0.0.1")
	}

	var hostnames []string
	var ipAddresses []net.IP
	seenHostnames := make(map[string]struct{}, len(configuredHosts))
	seenIPs := make(map[string]struct{}, len(configuredHosts))

	for _, host := range configuredHosts {
		if ip := net.ParseIP(host); ip != nil {
			key := ip.String()
			if _, ok := seenIPs[key]; ok {
				continue
			}
			seenIPs[key] = struct{}{}
			ipAddresses = append(ipAddresses, ip)
			continue
		}

		host = strings.TrimSpace(host)
		if host == "" {
			continue
		}
		if _, ok := seenHostnames[host]; ok {
			continue
		}
		seenHostnames[host] = struct{}{}
		hostnames = append(hostnames, host)
	}

	return hostnames, ipAddresses
}

func splitCommaSeparatedValues(value string) []string {
	parts := strings.Split(value, ",")
	values := make([]string, 0, len(parts))
	seen := make(map[string]struct{}, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if _, ok := seen[part]; ok {
			continue
		}
		seen[part] = struct{}{}
		values = append(values, part)
	}
	return values
}
