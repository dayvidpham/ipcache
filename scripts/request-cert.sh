#!/usr/bin/env sh

oci \
    certs-mgmt \
    certificate \
    create-certificate-managed-externally-issued-by-internal-ca \
        --name "dummy-placeholder" \
        --compartment-id "dummy-placeholder" \
        --description "dummy-placeholder" \
        --csr-pem "$(cat ../certs/dummy-placeholder.csr)" \
        --issuer-certificate-authority-id "dummy-placeholder"


