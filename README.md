# envoy_http2_tls_no_alpn

Proof of concept for a HTTP connection that terminates TLS at envoy, but
negotiates the HTTP version at the backend.

The "ideal" method would be:
1. Make a HTTP/1.1 + TLS request with an `Upgrade: h2c` header
1. Envoy terminates TLS and forwards the upgrade header to the backend
1. The backend either accepts or rejects the h2c upgrade:
    1. If the backend speaks h2c, it responds with `101 Switching Protocols` and
     starts sending HTTP/2 frames on the TCP connection in the response (see
     https://tools.ietf.org/html/rfc7540#section-3.2)
    1. If the backend only speaks HTTP/1.1, it does not switch protocols and the
     client continues with HTTP/1.1

Maybe it will work 🤞

Unfortunately, golang's `http2.Transport` doesn't handle the `h2c` upgrade flow
for us, so we have to get creative.

Current limitations:
1. Depends on fork of golang standard library (https://github.com/Gerg/net)

Current anti-limitations (previous limitations we resolved):
1. We no longer have to copy the HTTP/2 frame parsing logic from the `http2`
   package. Instead we offload that to https://github.com/Gerg/net
1. It works using either a `tls.Client` or a `httputil.ReverseProxy`
1. In the HTTP/2 case, it issues only a single request
1. Because it shares the TCP connection from the Upgrade request, it re-uses the same TCP connection
1. It no longer depends on ALPN to use HTTP/2 for the second connection


### Sneaky Client

Attempts to get a HTTP client to use the steps described above.

### Sneaky Reverse Proxy

Attempts to use `httputil.ReveseProxy` to serve HTTP/2 traffic and use the
steps above to communicate with the backend via the Envoy proxy.

## Installation

1. Install Envoy: https://www.envoyproxy.io/docs/envoy/latest/start/install#install
1. Install golang: https://golang.org/doc/install
1. 🎉

## Running

Testing HTTP/1.1 Backend:
1. `./start.sh` <- This will build and start the h2c app and envoy proxy and
   reverse proxy
Testing H2C Backend:
1. `H2C=true ./start.sh` <- This will build and start the http1 app and envoy proxy and reverse proxy

For sneaky client:
1. `./sneaky_client/sneaky-client` <- This will attempt the TLS + h2c upgrade request to envoy
1.  See that it works?

For sneaky reverse proxy:
1. `curl -k https://localhost:8000 --http2` <- This will make an HTTP/2 request
   to the reverse proxy, which will then attempt the TLS + h2c upgrade request
   to envoy
1.  See that it works?

## Ports

|Port|What|
|-|-|
|8080|H2C app OR HTTP/1.1 app, depending on H2C environment variable passed to start script|
|61001|Envoy|
|8000|Reverse Proxy|

## Debugging

Helpful for seeing what's going on:
`sudo ngrep -d any port 8080`

For more details, you can use `tshark` (terminal version of WireShark):
```
tshark -i lo -w /tmp/shark.pcap
....
tshark -r /tmp/shark.pcap -Y "(http or http2)" -T text -V
```

You can use the `GODEBUG=http2debug=2` environment variable for both the
client and server. This will show HTTP/2 frames sent and received.
