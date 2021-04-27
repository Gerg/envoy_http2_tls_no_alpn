#! /usr/bin/env bash

pushd ./http1_app
  go build
popd

pushd ./h2c_app
  go build
popd

pushd ./sneaky_client
  go build
popd

pushd ./sneaky_reverse_proxy
  go build
popd

trap 'kill $(jobs -p)' EXIT

echo "$H2C"

if [[ "$H2C" = "true" ]]; then
  echo "Starting h2c App on 8080"
  ./h2c_app/main.py &
else
  echo "Starting HTTP/1.1 App on 8080"
  ./http1_app/http1-app &
fi

cp ./envoy/sds-server-cert-and-key.yaml /tmp
cp ./envoy/sds-server-validation-context.yaml /tmp

# Hint: you need envoy
echo "Starting Envoy on 61001"
envoy --config-path ./envoy/envoy.yaml -l error &

echo "Starting Reverse Proxy on 8000"
./sneaky_reverse_proxy/sneaky_reverse_proxy &

echo "Run ./sneaky_client/sneaky-client"
wait
