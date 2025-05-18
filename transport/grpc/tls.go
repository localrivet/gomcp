package grpc

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"

	"google.golang.org/grpc/credentials"
)

// loadTLSCredentials loads TLS credentials for server or client.
func loadTLSCredentials(certFile, keyFile, caFile string, isServer bool) (credentials.TransportCredentials, error) {
	// Load certificate of the CA who signed server's certificate
	var certPool *x509.CertPool
	if caFile != "" {
		certPool = x509.NewCertPool()
		caBytes, err := os.ReadFile(caFile)
		if err != nil {
			return nil, fmt.Errorf("failed to read CA certificate: %w", err)
		}

		if !certPool.AppendCertsFromPEM(caBytes) {
			return nil, fmt.Errorf("failed to add CA certificate to pool")
		}
	}

	// Load server or client certificate and key
	var certificate tls.Certificate
	var err error
	if certFile != "" && keyFile != "" {
		certificate, err = tls.LoadX509KeyPair(certFile, keyFile)
		if err != nil {
			return nil, fmt.Errorf("failed to load certificate key pair: %w", err)
		}
	}

	// Create the credentials
	var config *tls.Config
	if isServer {
		config = &tls.Config{
			Certificates: []tls.Certificate{certificate},
			ClientAuth:   tls.RequireAndVerifyClientCert,
			ClientCAs:    certPool,
		}

		// If no CA is provided, don't require client cert verification
		if certPool == nil {
			config.ClientAuth = tls.NoClientCert
		}
	} else {
		config = &tls.Config{
			Certificates: []tls.Certificate{certificate},
			RootCAs:      certPool,
		}
	}

	return credentials.NewTLS(config), nil
}

// getServerTLSCredentials returns TLS credentials for the server.
func (t *Transport) getServerTLSCredentials() (credentials.TransportCredentials, error) {
	return loadTLSCredentials(t.tlsCertFile, t.tlsKeyFile, t.tlsCAFile, true)
}

// getClientTLSCredentials returns TLS credentials for the client.
func (t *Transport) getClientTLSCredentials() (credentials.TransportCredentials, error) {
	return loadTLSCredentials(t.tlsCertFile, t.tlsKeyFile, t.tlsCAFile, false)
}
