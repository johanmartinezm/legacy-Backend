#!/bin/bash
echo "Starting Go Server..."
# Forzar a Go a usar un directorio de trabajo local si /tmp falla
mkdir -p tmp
export GOTMPDIR=./tmp
go run cmd/server/main.go


