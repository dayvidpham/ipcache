#!/usr/bin/env sh

# Stop on first fail
set -e

go build ./server.go
go build ./client.go
