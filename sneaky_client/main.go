package main

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"strconv"

	"golang.org/x/net/http2"
	"golang.org/x/net/http2/hpack"
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
		// https://github.com/golang/net/blob/d25e3042586827419b3589d4a4697231930a15d6/http2/hpack/encode.go#L13
		initialHeaderTableSize := uint32(4096)
		// https://github.com/golang/net/blob/d25e3042586827419b3589d4a4697231930a15d6/http2/transport.go#L39
		transportDefaultConnFlow := uint32(1 << 30)
		transportDefaultStreamFlow := uint32(4 << 20)

		// https://github.com/golang/net/blob/d25e3042586827419b3589d4a4697231930a15d6/http2/transport.go#L640
		bw := bufio.NewWriter(conn)
		framer := http2.NewFramer(bw, br)
		framer.ReadMetaHeaders = hpack.NewDecoder(initialHeaderTableSize, nil)

		initialSettings := []http2.Setting{
			{ID: http2.SettingEnablePush, Val: 0},
			{ID: http2.SettingInitialWindowSize, Val: transportDefaultStreamFlow},
		}

		// https://github.com/golang/net/blob/d25e3042586827419b3589d4a4697231930a15d6/http2/transport.go#L695
		bw.Write([]byte(http2.ClientPreface))
		framer.WriteSettings(initialSettings...)
		framer.WriteWindowUpdate(0, transportDefaultConnFlow)
		bw.Flush()

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
				// https://github.com/golang/net/blob/d25e3042586827419b3589d4a4697231930a15d6/http2/transport.go#L1957
				status := frame.PseudoValue("status")
				statusCode, _ := strconv.Atoi(status)

				regularFields := frame.RegularFields()
				strs := make([]string, len(regularFields))
				header := make(http.Header, len(regularFields))
				resp = &http.Response{
					Proto:      "HTTP/2.0",
					ProtoMajor: 2,
					Header:     header,
					StatusCode: statusCode,
					Status:     status + " " + http.StatusText(statusCode),
				}
				for _, hf := range regularFields {
					key := http.CanonicalHeaderKey(hf.Name)
					vv := header[key]
					if vv == nil && len(strs) > 0 {
						vv, strs = strs[:1:1], strs[1:]
						vv[0] = hf.Value
						header[key] = vv
					} else {
						header[key] = append(vv, hf.Value)
					}
				}
				if frame.StreamEnded() {
					fmt.Println("Stream Ended")
					break loop
				}
			case *http2.DataFrame:
				fmt.Printf("Got DATA: %+v\n", frame)
				// https://github.com/golang/net/blob/d25e3042586827419b3589d4a4697231930a15d6/http2/transport.go#L2195
				data := frame.Data()
				fmt.Printf("Response Data: %+v\n", string(frame.Data()))
				body := &buffer{}
				resp.Body = body
				_, err = body.Write(data)
				if err != nil {
					log.Fatalf("Writing body failed %v\n", err)
				}
				if frame.StreamEnded() {
					fmt.Println("Stream Ended")
					break loop
				}
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
			default:
				log.Fatalf("Transport: unhandled response frame type %T", frame)
			}

			i++
		}
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
