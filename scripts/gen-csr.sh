#!/usr/bin/env sh

openssl req \
    -new \
    -key ../certs/dummy-placeholder.key \
    -inform PEM \
    -config ./minimal_csr.cnf \
    -outform PEM \
    -out ./certs/dummy-placeholder.csr

openssl req \
    -verify \
    -in ../certs/dummy-placeholder.csr \
    -text \
    -pubkey \
    -noout
