package main

import (
	"bufio"
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httputil"
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
	// Create a server on port 8000
	// Exactly how you would run an HTTP/1.1 server
	srv := &http.Server{Addr: ":8000", Handler: http.HandlerFunc(handle)}

	// Start the server with TLS, since we are running HTTP/2 it must be
	// run with TLS.
	// Exactly how you would run an HTTP/1.1 server with TLS connection.
	log.Printf("Serving on https://0.0.0.0:8000")
	log.Fatal(srv.ListenAndServeTLS("./sneaky_reverse_proxy/server.crt", "./sneaky_reverse_proxy/server.key"))
}

type SneakyTransport struct {
	TLSClientConfig *tls.Config
}

func (t *SneakyTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	transport := &http.Transport{TLSClientConfig: t.TLSClientConfig}
	resp, err := transport.RoundTrip(req)

	if err != nil {
		return nil, err
	}
	if resp.StatusCode == 101 {
		fmt.Println("SneakyTransport Handling upgrade")
		// https://github.com/golang/net/blob/d25e3042586827419b3589d4a4697231930a15d6/http2/hpack/encode.go#L13
		initialHeaderTableSize := uint32(4096)
		// https://github.com/golang/net/blob/d25e3042586827419b3589d4a4697231930a15d6/http2/transport.go#L39
		transportDefaultConnFlow := uint32(1 << 30)
		transportDefaultStreamFlow := uint32(4 << 20)

		defer resp.Body.Close()
		rw := resp.Body.(io.ReadWriteCloser)

		// https://github.com/golang/net/blob/d25e3042586827419b3589d4a4697231930a15d6/http2/transport.go#L640
		bw := bufio.NewWriter(rw)
		framer := http2.NewFramer(bw, rw)
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
		body := &buffer{}

		// https://github.com/golang/net/blob/d25e3042586827419b3589d4a4697231930a15d6/http2/transport.go#L1818
	loop:
		for {
			frame, err := framer.ReadFrame()
			if err != nil {
				fmt.Printf("Read frame %d failed %v\n", i, err)
				return nil, err
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
				resp.Body = body
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
				_, err = body.Write(data)
				if err != nil {
					fmt.Printf("Writing body failed %v\n", err)
					return nil, err
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

	fmt.Printf("SneakyTransport response: %+v\n", resp)

	return resp, err
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

	transport = &SneakyTransport{TLSClientConfig: tlsConfig}

	proxy.Transport = transport

	proxy.ServeHTTP(w, r)
}
