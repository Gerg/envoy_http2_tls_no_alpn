package main

import (
	"bufio"
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"

	"golang.org/x/net/http2"
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

	dialFn := http.DefaultTransport.(*http.Transport).DialContext

	conn, err := dialFn(context.Background(), "tcp", "localhost:61001")
	if err != nil {
		log.Fatalf("Error Dial %v\n", err)
	}
	conn = tls.Client(conn, tlsConfig)

	req, err := http.NewRequest("GET", "https://localhost:61001", nil)
	if err != nil {
		log.Fatalf("Error Request %v\n", err)
	}
	req.Header.Set("Connection", "Upgrade, HTTP2-Settings")
	req.Header.Set("Upgrade", "h2c")
	req.Header.Set("HTTP2-Settings", "AAMAAABkAARAAAAAAAIAAAAA")
	req.Header.Set("X-Debug", "Upgrade Request")

	fmt.Printf("Request: %+v\n", req)

	if err := req.Write(conn); err != nil {
		log.Fatalf("Error write request %v\n", err)
	}

	br := bufio.NewReader(conn)

	resp, err := http.ReadResponse(br, req)
	if err != nil {
		log.Fatalf("Error read response %v\n", err)
	}
	fmt.Printf("Response: %+v\n", resp)

	if resp.StatusCode == 101 {
		// https://github.com/golang/net/blob/0fccb6fa2b5ce302a9da5afc2513d351bd175889/http2/transport.go#L678-L680
		http2Transport := &http2.Transport{AllowHTTP: true}
		_, err := http2Transport.NewClientConn(conn)
		if err != nil {
			log.Fatalf("HTTP/2 Conn error %v\n", err)
		}

		bw := bufio.NewWriter(conn)
		framer := http2.NewFramer(bw, br)

		i := 0

		// https://github.com/golang/net/blob/d25e3042586827419b3589d4a4697231930a15d6/http2/transport.go#L1818
	loop:
		for {
			frame, err := framer.ReadFrame()
			if err != nil {
				log.Fatalf("Read frame %d failed %v\n", i, err)
			}
			fmt.Printf("FRAME %d: %+v\n", i, frame)

			switch frame := frame.(type) {
			case *http2.MetaHeadersFrame:
				fmt.Printf("Got HEADERS: %+v\n", frame)
				fmt.Printf("Response Headers: %+v\n", frame.String())
			case *http2.DataFrame:
				fmt.Printf("Got DATA: %+v\n", frame)
				fmt.Printf("Response Data: %+v\n", string(frame.Data()))
			case *http2.GoAwayFrame:
				fmt.Printf("Got GOAWAY: %+v\n", frame)
				break loop
			case *http2.RSTStreamFrame:
				fmt.Printf("Got RST_STREAM: %+v\n", frame)
				break loop
			case *http2.SettingsFrame:
				fmt.Printf("Got SETTINGS: %+v\n", frame)
				if !frame.IsAck() {
					fmt.Println("ACKing SETTINGS", frame)
					framer.WriteSettingsAck()
				}
			case *http2.PushPromiseFrame:
				fmt.Printf("Got PUSH_PROMISE: %+v\n", frame)
			case *http2.WindowUpdateFrame:
				fmt.Printf("Got WINDOW_UPDATE: %+v\n", frame)
			case *http2.PingFrame:
				fmt.Printf("Got PING: %+v\n", frame)
			}

			i++
		}
	}

	fmt.Printf("Client Proto: %d\n", resp.ProtoMajor)
	fmt.Printf("%+v\n", resp)
	fmt.Printf("%+v\n", resp.Header)
}
