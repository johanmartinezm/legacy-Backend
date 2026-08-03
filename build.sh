#!/bin/bash
echo "Building Go Server..."
mkdir -p tmp
export GOTMPDIR=./tmp
go build -o server cmd/server/main.go
if [ $? -eq 0 ]; then
  echo "Build successful! Executable created: ./server"
else
  echo "Build failed!"
  exit 1
fi

