package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"database/sql"
	"encoding/base64"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"time"

	"github.com/dayvidpham/ipcache/internal/msgs"
	"github.com/mattn/go-sqlite3"
)

var (
	parsedTlsHandshakeTimeoutSeconds uint
	parsedPingTimeoutSeconds         uint

	tlsHandshakeTimeout time.Duration
	pingTimeout         time.Duration
)

func init() {
	flag.UintVar(&parsedTlsHandshakeTimeoutSeconds, "tls-handshake-timeout-seconds", 5, "max time to complete TLS handshake before server kills connection")
	flag.UintVar(&parsedPingTimeoutSeconds, "ping-timeout-seconds", 60*10, "max time between pings to server for daemons, before server kills connection")
}

func main() {
	log.SetFlags(log.Lshortfile)

	///////////////////////////////
	// Proccess flags
	///////////////////////////////
	flag.Parse()
	log.Println("[DEBUG] --tls-handshake-timeout-seconds", parsedTlsHandshakeTimeoutSeconds)
	log.Println("[DEBUG] --ping-timeout-seconds", parsedPingTimeoutSeconds)

	pingTimeout = time.Second * time.Duration(parsedPingTimeoutSeconds)
	tlsHandshakeTimeout = time.Second * time.Duration(parsedTlsHandshakeTimeoutSeconds)

	///////////////////////////////
	// Establish connection to SQLite3 DB
	///////////////////////////////
	sql3V, _, _ := sqlite3.Version()
	log.Printf("[DEBUG] sqlite3 version: %v\n", sql3V)

	rootCtx := context.Background()
	db, err := sql.Open("sqlite3", "file:ipcache.db")
	if err != nil {
		log.Println(err)
		return
	}
	defer db.Close()

	dbTimeout := time.Second * 3
	pingCtx, cancel := context.WithTimeout(rootCtx, dbTimeout)
	defer cancel()
	if err = db.PingContext(pingCtx); err != nil {
		log.Println(err)
		return
	}

	initCtx, cancel := context.WithTimeout(rootCtx, dbTimeout)
	defer cancel()
	if err = initDb(initCtx, db); err != nil {
		log.Println(err)
		return
	}

	var rows *sql.Rows
	if rows, err = db.Query(
		`SELECT
			*
		FROM
			AuthorizationType;`); err != nil {
		log.Println(err)
		return
	}

	defer rows.Close()
	for rows.Next() {
		var (
			c1 int
			c2 string
		)

		err = rows.Scan(&c1, &c2)
		if err != nil {
			log.Println(err)
			return
		}
		log.Println(c1, c2)
	}

	if err = rows.Err(); err != nil {
		log.Println(err)
		return
	}

	///////////////////////////////
	// Read in certificates, CA bundles
	// Referencing https://smallstep.com/hello-mtls/doc/combined/go/go
	// Referencing https://gist.github.com/denji/12b3a568f092ab951456
	///////////////////////////////
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
	// sync.Map
	cache := make(map[string]bool)
	config := &tls.Config{
		MinVersion:   tls.VersionTLS13,
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

	///////////////////////////////
	// Start server
	///////////////////////////////
	ln, err := tls.Listen("tcp", "127.0.0.1:4430", config)
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
		log.Println("[INFO] New tls.Conn established with", conn.RemoteAddr(), ", still need to do TLS handshake")

		go TlsServe(conn)
	}
}

func TlsServe(conn *tls.Conn) {
	defer conn.Close()

	var (
		err     error
		client  msgs.Client
		recvMsg msgs.Message
	)

	// TLS timeout
	err = conn.SetDeadline(time.Now().Add(tlsHandshakeTimeout))
	if err != nil {
		log.Println("[ERROR] Failed to set a timeout for TLS handshake, rejecting conn.\n\t-", err)
		return
	}
	// NOTE: The TLS handshake is normally performed lazily, but do eagerly to fail fast
	if err = conn.Handshake(); err != nil {
		log.Println("[ERROR] Failed TLS handshake for", conn.RemoteAddr(), ".\n\t- Reason:", err)
		return
	}
	// Unset TLS timeout
	if err = conn.SetDeadline(time.Time{}); err != nil {
		log.Println("[ERROR] Failed to unset timeout for TLS handshake.\n\t-", err)
		return
	}
	log.Println("[INFO] TLS handshake succeeded!")

	if client, err = msgs.NewClient(conn); err != nil {
		log.Println(err)
		return
	}

	// NOTE: Need some kind of session identifier next???
	// Side-effect from VerifyConnection to tell us client's SubjectKeyId/pubkey/session?
	// func GetConnPubkey(conn *tls.Conn) { ... }

	server := msgs.NewMessenger(conn)

	//sendMsg := msgs.String("Hello from server\n")
	//n, err := server.SendN(sendMsg)
	//log.Printf("[INFO] Sending message of %d bytes, %d bytes sitting in buffer\n", sendMsg.Size(), n)
	//if err != nil {
	//	log.Println(err)
	//	return
	//}

	for {
		recvMsg, err = server.Receive()
		switch err {
		case nil:
			break
		case io.EOF:
			log.Println("Connection closed")
			return
		default:
			log.Println(err)
			return
		}

		log.Printf("Received from %+v: %s\n", client, recvMsg.Type())

		err = nil
		switch recvMsg.Type() {
		case msgs.T_String:
			StringMessageHandler(recvMsg)

		case msgs.T_ClientRegister:
			// On register:
			// - [x] respond with server's ping timeout interval
			// - [ ] store IP in DB
			//   - [ ] handle logic if IP already in
			//   - [ ] handle logic if IP in and different
			err = ClientRegisterHandler(server, pingTimeout)
		case msgs.T_Ping:
			err = PingHandler(server, pingTimeout)

		default:
			err = fmt.Errorf("[ERROR] Unimplemented message type:\n\t- %s\n", recvMsg.Type())
		}

		if err != nil {
			log.Println(err)
			return
		}
	}
}

func StringMessageHandler(recvMsg msgs.Message) {
	log.Printf("\t- Payload: %s\n", recvMsg.Payload())
}

func ClientRegisterHandler(server msgs.Messenger, pingTimeout time.Duration) (err error) {
	log.Printf("\t- Responding with ping timeout as String(%v) ...\n", pingTimeout)

	timeoutMsg := msgs.String(pingTimeout.String())
	if err = server.Send(timeoutMsg); err != nil {
		return err
	}
	log.Printf("\t- Sent response. Resetting SetReadTimeout(%v).\n\n", pingTimeout)

	err = server.SetReadTimeout(pingTimeout)
	if err != nil {
		return err
	}
	return
}

func PingHandler(server msgs.Messenger, pingTimeout time.Duration) (err error) {
	log.Printf("\t- Resetting SetReadTimeout(%v).\n\n", pingTimeout)
	err = server.SetReadTimeout(pingTimeout)
	return err
}
