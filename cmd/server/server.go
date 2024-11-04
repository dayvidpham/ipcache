package main

import (
	"bufio"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"errors"
	"io"
	"log"

	//"net"
	"os"
	//"database/sql"
	//"github.com/mattn/go-sqlite3"
)

func main() {
	// Referencing https://gist.github.com/denji/12b3a568f092ab951456
	log.SetFlags(log.Lshortfile)
	log.Println("Hello, World!")

	// From https://smallstep.com/hello-mtls/doc/combined/go/go
	caCert, _ := os.ReadFile("certs/self.pem")
	caCertPool := x509.NewCertPool()
	caCertPool.AppendCertsFromPEM(caCert)

	cert, err := tls.LoadX509KeyPair("./certs/self.pem", "./certs/self.key")
	if err != nil {
		panic(err)
	}

	// BUG: Will need to synchronize for concurrent r/w
	// mutex: sync.RWLock
	// atomic CAS: atomics
	cache := make(map[string]bool)
	config := &tls.Config{
		Certificates: []tls.Certificate{cert},
		ClientCAs:    caCertPool,
		ClientAuth:   tls.RequireAndVerifyClientCert,
		VerifyConnection: func(state tls.ConnectionState) (err error) {
			// NOTE: Use this b/c need entrypoint that's always called in order to grab client's cert
			certs := state.PeerCertificates
			if len(certs) < 1 {
				return errors.New("PeerCertificates is empty, none were given by client")
			}
			cert := certs[0]

			pubkey := base64.StdEncoding.EncodeToString(cert.SubjectKeyId)
			_, ok := cache[pubkey]
			if !ok {
				cache[pubkey] = true
			}
			log.Printf("[INFO] Client public key: %+v\n", pubkey)

			return err
		},
	}

	ln, err := tls.Listen("tcp", ":4430", config)
	if err != nil {
		panic(err)
	}
	defer ln.Close()

	for {
		netconn, err := ln.Accept()
		if err != nil {
			log.Println("[ERROR] Failed to accept connection\n\t-", err)
			continue
		}

		conn, ok := netconn.(*tls.Conn)
		if !ok {
			log.Println("[ERROR] Connection established but failed tls.Conn type assert.")
			netconn.Close()
			continue
		}
		log.Println("[INFO] New tls.Conn established with", conn.RemoteAddr().String(), ", still need to do TLS handshake")

		go TlsServe(conn)
	}

}

func TlsServe(conn *tls.Conn) {
	defer conn.Close()

	// NOTE: The TLS handshake is normally performed lazily, but do eagerly to fail fast
	err := conn.Handshake()
	if err != nil {
		log.Println("[ERROR] Failed TLS handshake.\n\t- Reason:", err)
		return
	}
	log.Println("[INFO] TLS handshake succeeded!")

	// NOTE: Need some kind of session identifier next???
	// Side-effect from VerifyConnection to tell us client's SubjectKeyId/pubkey/session?
	// func GetConnPubkey(conn *tls.Conn) { ... }

	r, w := bufio.NewReader(conn), bufio.NewWriter(conn)
	rw := bufio.NewReadWriter(r, w)

	n, err := rw.Write([]byte("Hello from server\n"))
	log.Println("Buffered", n, "bytes")
	if err != nil {
		log.Println("\tGot error:", err)
		return
	}

	err = rw.Flush()
	if err != nil {
		log.Println("\tGot error:", err)
		return
	}

	for {
		msg, err := rw.ReadString('\n')
		switch err {
		case nil:
			break
		case io.EOF:
			log.Println("Connection closed")
			return
		default:
			log.Println("Call to ReadString failed\n\t- Reason:", err)
			return
		}
		log.Print("Received from client: ", msg)
	}

}
