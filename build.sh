#!/usr/bin/env sh

# Stop on first fail
set -e

go build ./cmd/server
go build ./cmd/client
go build ./cmd/clientd
