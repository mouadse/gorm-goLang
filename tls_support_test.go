package main

import (
	"crypto/x509"
	"encoding/pem"
	"os"
	"slices"
	"testing"
)

func TestResolveTLSCertificateFilesGeneratesSelfSignedPair(t *testing.T) {
	t.Setenv(defaultTLSCertPathEnv, "")
	t.Setenv(defaultTLSKeyPathEnv, "")
	t.Setenv(defaultTLSHostsEnv, "localhost,app,127.0.0.1")

	certFile, keyFile, err := resolveTLSCertificateFiles()
	if err != nil {
		t.Fatalf("resolveTLSCertificateFiles: %v", err)
	}

	certPEM, err := os.ReadFile(certFile)
	if err != nil {
		t.Fatalf("read cert file: %v", err)
	}
	keyPEM, err := os.ReadFile(keyFile)
	if err != nil {
		t.Fatalf("read key file: %v", err)
	}

	certBlock, _ := pem.Decode(certPEM)
	if certBlock == nil {
		t.Fatal("expected PEM certificate block")
	}
	certificate, err := x509.ParseCertificate(certBlock.Bytes)
	if err != nil {
		t.Fatalf("parse certificate: %v", err)
	}

	if !slices.Contains(certificate.DNSNames, "localhost") {
		t.Fatalf("expected localhost SAN, got %v", certificate.DNSNames)
	}
	if !slices.Contains(certificate.DNSNames, "app") {
		t.Fatalf("expected app SAN, got %v", certificate.DNSNames)
	}
	if got := certificate.IPAddresses[0].String(); got != "127.0.0.1" {
		t.Fatalf("expected first IP SAN 127.0.0.1, got %s", got)
	}

	keyBlock, _ := pem.Decode(keyPEM)
	if keyBlock == nil {
		t.Fatal("expected PEM private key block")
	}
}

func TestResolveTLSCertificateFilesRequiresCertAndKeyTogether(t *testing.T) {
	t.Setenv(defaultTLSCertPathEnv, "/tmp/cert.pem")
	t.Setenv(defaultTLSKeyPathEnv, "")

	if _, _, err := resolveTLSCertificateFiles(); err == nil {
		t.Fatal("expected error when only one TLS path is configured")
	}
}
