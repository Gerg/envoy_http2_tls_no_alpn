package main

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"

	"golang.org/x/net/http2"
)

func main() {
	client := http.Client{}

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
		TLSClientConfig: tlsConfig,
	}

	client.Transport = transport
	req, err := http.NewRequest("GET", "https://localhost:61001", nil)
	if err != nil {
		fmt.Printf("Error %v\n", err)
	}
	req.Header.Set("Connection", "Upgrade, HTTP2-Settings")
	req.Header.Set("Upgrade", "h2c")
	req.Header.Set("HTTP2-Settings", "AAMAAABkAARAAAAAAAIAAAAA")

	fmt.Printf("%+v\n", req)
	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("Error %v\n", err)
	}

	if resp.StatusCode == 101 {
		resp.Body.Close()
		client.Transport = &http2.Transport{
			TLSClientConfig: tlsConfig,
		}

		req, err := http.NewRequest("GET", "https://localhost:61001", nil)
		if err != nil {
			fmt.Printf("2nd Request Error %v\n", err)
		}

		resp, err = client.Do(req)
		if err != nil {
			fmt.Printf("2nd Request Error %v\n", err)
		}
	}
	defer resp.Body.Close()

	fmt.Printf("Client Proto: %d\n", resp.ProtoMajor)
	fmt.Printf("%+v\n", resp)
	fmt.Printf("%+v\n", resp.Header)
}
