package main

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"log"

	"github.com/gerg/net/http"
)

func main() {
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

	transport := &http.Transport{
		TLSClientConfig:   tlsConfig,
		ForceAttemptHTTP2: true,
	}

	client := http.Client{
		Transport: transport,
	}

	req, err := http.NewRequest("GET", "https://localhost:61001", nil)
	if err != nil {
		log.Fatalf("Error making new request: %+v\n", err)
	}

	req.Header.Set("Connection", "Upgrade, HTTP2-Settings")
	req.Header.Set("Upgrade", "h2c")
	req.Header.Set("HTTP2-Settings", "AAMAAABkAARAAAAAAAIAAAAA")
	req.Header.Set("X-Debug", "Upgrade Request")

	resp, err := client.Do(req)
	if err != nil {
		log.Fatalf("Error sending request: %+v\n", err)
	}

	fmt.Print("\n\n====== Response ======\n\n")
	fmt.Printf("Raw Response:\n\t%+v\n\n", resp)
	fmt.Printf("Client Proto:\n\t%d\n\n", resp.ProtoMajor)
	fmt.Printf("Headers:\n\t%+v\n\n", resp.Header)

	defer resp.Body.Close()

	bodyBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Fatalf("Error reading resp body: %+v\n", err)
	}
	bodyString := string(bodyBytes)

	fmt.Printf("Response Body:\n\t%+v\n", bodyString)
}
