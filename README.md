# envoy_http2_tls_no_alpn
Proof of concept for a HTTP connection that terminates TLS at envoy, but
negotiates the HTTP version at the backend. Maybe impossible.

The "ideal" method would be:
1. Make a HTTP/1.1 + TLS request with an `Upgrade: h2c` header
1. Envoy terminates TLS and forwards the upgrade header to the backend
1. The backend either accepts or rejects the h2c upgrade:
  1. If the backend speaks h2c, it responds with `101 Switching Protocols` and
     the client sends a HTTP/2 request
  1. If the backend only speaks HTTP/1.1, it does not switch protocols and the
     client continues with HTTP/1.1

Maybe it will work 🤞

Current limitations:
1. In the HTTP/2 case, it issues multiple requests
1. Because it uses a different transport for the second upgrade, it doesn't re-use the TCP connection
1. It still depends on ALPN to use HTTP/2 for the second connection
1. It is currently using a `http.Client`, and the same techniques might not work for a `httputil.ReverseProxy`
1. Probably some other stuff

Ideally, we could find a way to re-use the same Transport and/or connection for
the second request. Even better, we could find a way to make the `http.Client`
respect the `101 Switching Protocols` for us, like it does when using regular
h2c. Hopefully in either of those cases, we can remove ALPN support from the
envoy config.

## Installation

1. Install Envoy: https://www.envoyproxy.io/docs/envoy/latest/start/install#install
1. Install golang: https://golang.org/doc/install
1. 🎉

## Running

Testing HTTP/1.1 Backend:
1. `./start.sh` <- This will build and start the h2c app and envoy proxy
Testing H2C Backend:
1. `H2C=true ./start.sh` <- This will build and start the http1 app and envoy proxy

Either Way:
1. `./sneaky_client/sneaky-client` <- This will attempt the TLS + h2c upgrade request to envoy
1.  See that it works?

Helpful for seeing what's going on:
`sudo ngrep -d any port 8080`
