package main

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"io/ioutil"
	"log"

	"github.com/gerg/net/http/httputil"

	"github.com/gerg/net/http"
)

type buffer struct {
	bytes.Buffer
}

// Add a Close method to our buffer so that we satisfy io.ReadWriteCloser.
func (b *buffer) Close() error {
	b.Buffer.Reset()
	return nil
}

func main() {
	// Create a server on port 8000
	// Exactly how you would run an HTTP/1.1 server
	srv := &http.Server{Addr: ":8000", Handler: http.HandlerFunc(handle)}

	// Start the server with TLS, since we are running HTTP/2 it must be
	// run with TLS.
	// Exactly how you would run an HTTP/1.1 server with TLS connection.
	log.Printf("Serving on https://0.0.0.0:8000")
	log.Fatal(srv.ListenAndServeTLS("./sneaky_reverse_proxy/server.crt", "./sneaky_reverse_proxy/server.key"))
}

func handle(w http.ResponseWriter, r *http.Request) {
	envoyHost := "localhost:61001"
	director := func(req *http.Request) {
		req.Header.Add("X-Forwarded-Host", req.Host)
		req.Header.Add("X-Origin-Host", envoyHost)
		req.Header.Add("Upgrade", "h2c")
		req.Header.Add("HTTP2-Settings", "AAMAAABkAARAAAAAAAIAAAAA")
		req.Header.Add("Connection", "Upgrade, HTTP2-Settings")
		req.Header.Add("X-Debug", "Upgrade Request")
		req.URL.Scheme = "https"
		req.URL.Host = envoyHost
	}

	proxy := &httputil.ReverseProxy{Director: director}

	var transport http.RoundTripper

	caCert, err := ioutil.ReadFile("./client_certs/ca.crt")
	if err != nil {
		log.Fatalf("Reading CA: %s\n", err)
	}
	caCertPool := x509.NewCertPool()
	caCertPool.AppendCertsFromPEM(caCert)
	cert, err := tls.LoadX509KeyPair("./client_certs/client.crt", "./client_certs/client.key")
	if err != nil {
		log.Fatalf("Reading cert/key: %s\n", err)
	}

	tlsConfig := &tls.Config{
		RootCAs:            caCertPool,
		InsecureSkipVerify: true,
		Certificates:       []tls.Certificate{cert},
	}

	transport = &http.Transport{
		TLSClientConfig:   tlsConfig,
		ForceAttemptHTTP2: true,
	}

	proxy.Transport = transport

	proxy.ServeHTTP(w, r)
}
