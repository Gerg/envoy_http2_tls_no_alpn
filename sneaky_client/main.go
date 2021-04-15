package main

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"time"

	"golang.org/x/net/http2"
)

type connWrapper struct {
	io.ReadWriteCloser
}

func (c connWrapper) LocalAddr() net.Addr {
	return nil
}

func (c connWrapper) RemoteAddr() net.Addr {
	return nil
}

func (c connWrapper) SetDeadline(t time.Time) error {
	return nil
}

func (c connWrapper) SetReadDeadline(t time.Time) error {
	return nil
}

func (c connWrapper) SetWriteDeadline(t time.Time) error {
	return nil
}

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
		log.Fatalf("Error Request %v\n", err)
	}
	req.Header.Set("Connection", "Upgrade, HTTP2-Settings")
	req.Header.Set("Upgrade", "h2c")
	req.Header.Set("HTTP2-Settings", "AAMAAABkAARAAAAAAAIAAAAA")
	req.Header.Set("X-Debug", "Upgrade Request")

	fmt.Printf("%+v\n", req)
	resp, err := client.Do(req)
	if err != nil {
		log.Fatalf("Error Do %v\n", err)
	}

	if resp.StatusCode == 101 {
		// defer resp.Body.Close()
		// body, err := io.ReadAll(resp.Body)

		// if err != nil {
		// 	log.Fatalf("Read Body Error %v\n", err)
		// } else {
		// 	fmt.Printf("First Response Body %v\n", body)
		// }

		backConn, ok := resp.Body.(io.ReadWriteCloser)
		if !ok {
			log.Fatalf("BackConn type assertion %v\n", err)
		}

		var wrapper net.Conn
		wrapper = connWrapper{backConn}

		// https://github.com/golang/net/blob/0fccb6fa2b5ce302a9da5afc2513d351bd175889/http2/transport.go#L678-L680
		http2Transport := &http2.Transport{AllowHTTP: true}
		_, err := http2Transport.NewClientConn(wrapper)
		if err != nil {
			log.Fatalf("HTTP/2 Conn error %v\n", err)
		}

		framer := http2.NewFramer(resp.Body.(io.Writer), resp.Body)
		// err = framer.WriteSettingsAck()
		// if err != nil {
		// 	log.Fatalf("Ack failed %v\n", err)
		// }

		frame, err := framer.ReadFrame()
		if err != nil {
			log.Fatalf("Read frame 1 failed %v\n", err)
		}
		fmt.Printf("FRAME 1: %+v\n", frame)

		frame, err = framer.ReadFrame()
		if err != nil {
			log.Fatalf("Read frame 2 failed %v\n", err)
		}
		fmt.Printf("FRAME 2: %+v\n", frame)

		frame, err = framer.ReadFrame()
		if err != nil {
			log.Fatalf("Read frame 3 failed %v\n", err)
		}
		fmt.Printf("FRAME 3: %+v\n", frame)

		frame, err = framer.ReadFrame()
		if err != nil {
			log.Fatalf("Read frame 4 failed %v\n", err)
		}
		fmt.Printf("FRAME 4: %+v\n", frame)

		frame, err = framer.ReadFrame()
		if err != nil {
			log.Fatalf("Read frame 5 failed %v\n", err)
		}
		fmt.Printf("FRAME 5: %+v\n", frame)
	}

	fmt.Printf("Client Proto: %d\n", resp.ProtoMajor)
	fmt.Printf("%+v\n", resp)
	fmt.Printf("%+v\n", resp.Header)
}
