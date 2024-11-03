!/usr/bin/env sh

openssl req -x509 -newkey rsa:4096 -keyout self.key -out self.pem -sha256 -days 3650 -nodes -config ./self_signed.cnf

