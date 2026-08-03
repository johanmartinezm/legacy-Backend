#!/bin/bash
echo "Building Go Server for Linux (amd64)..."
mkdir -p tmp
export GOTMPDIR=./tmp
# CGO_ENABLED=0 genera un binario estático sin dependencias externas
# GOOS=linux especifica el sistema operativo destino
# GOARCH=amd64 especifica la arquitectura (usualmente estándar en servidores)
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o server_linux cmd/server/main.go
if [ $? -eq 0 ]; then
  echo "Build successful! Linux executable created: ./server_linux"
else
  echo "Build failed!"
  exit 1
fi

