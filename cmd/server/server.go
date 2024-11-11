package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"database/sql"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"reflect"
	"sync"
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

var (
	rootCtx   context.Context
	db        *sql.DB
	dbTimeout time.Duration
	registrar *sync.Map
	daemons   *sync.Map
)

func init() {
	log.SetFlags(log.Lshortfile)

	flag.UintVar(&parsedTlsHandshakeTimeoutSeconds, "tls-handshake-timeout-seconds", 5, "max time to complete TLS handshake before server kills connection")
	flag.UintVar(&parsedPingTimeoutSeconds, "ping-timeout-seconds", 60*10, "max time between pings to server for daemons, before server kills connection")

	daemons = &sync.Map{}
}

func main() {
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

	var err error
	rootCtx = context.Background()
	db, err = sql.Open("sqlite3", "file:ipcache.db")
	if err != nil {
		log.Println(err)
		return
	}
	defer db.Close()

	dbTimeout = time.Second * 3
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

	registrarCtx, cancel := context.WithTimeout(rootCtx, dbTimeout)
	defer cancel()
	registrar, err = getRegistrar(registrarCtx, db)
	if err != nil {
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
	config := &tls.Config{
		MinVersion:   tls.VersionTLS13,
		Certificates: []tls.Certificate{cert},
		ClientCAs:    caCertPool,
		ClientAuth:   tls.RequireAndVerifyClientCert,
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
		//isDaemon   = false
		//isFirstMsg = true
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

	// TODO: Need some kind of session identifier next???
	// Side-effect from VerifyConnection to tell us client's SubjectKeyId/pubkey/session?
	// func GetConnPubkey(conn *tls.Conn) { ... }

	server := msgs.NewMessenger(conn)

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
		log.Printf("Received from %+v: %s\n", client, recvMsg.Type)

		err = nil
		switch recvMsg.Type {
		case msgs.T_String:
			log.Printf("\t- Payload: %s\n", recvMsg.Payload)
		case msgs.T_DaemonRegister:
			err = DaemonRegisterHandler(server, pingTimeout, client, recvMsg)
			defer deleteDaemon(client, err)
		case msgs.T_Ping:
			err = PingHandler(server, pingTimeout)

		default:
			err = fmt.Errorf("[ERROR] Unimplemented message type:\n\t- %s\n", recvMsg.Type)
		}

		if err != nil {
			log.Println(err)
			return
		}

	}
}

func DaemonRegisterHandler(
	server msgs.Messenger,
	pingTimeout time.Duration,
	client msgs.Client,
	recvMsg msgs.Message,
) (err error) {
	// Handles duplicate ID and IP registration
	var ipstr string
	if val, ok := daemons.Load(client.Id); ok {
		// Type-check `any` value
		ipstr, ok = val.(string)
		if !ok {
			log.Printf("[ERROR] Expected value with type `string` to be stored in `daemons`\n\t- Got %v: %+v\n", reflect.TypeOf(ipstr), ipstr)
		}

		// Found duplicate: terminate connection
		if ipstr == client.IP.String() {
			err = fmt.Errorf("[ERROR] Rejecting new registration for same client ID and IP\n\t- Client: %+v\n", client)

			errMsg := msgs.Err()
			errMsg.Payload = []byte(err.Error())
			sendErr := server.Send(errMsg)
			if sendErr != nil {
				err = fmt.Errorf("[ERROR] Failed to send errMsg to client: %w\n\t- %w\n", err, sendErr)
			}
			return err
		}
	} else {
		// Not duplicated
		ipstr = client.IP.String()
		daemons.Store(client.Id, ipstr)
	}

	registrarCtx, cancel := context.WithTimeout(rootCtx, dbTimeout)
	defer cancel()
	err = insertRegistrar(registrarCtx, db, client, recvMsg.UnixTimestampUtc)
	if err != nil {
		return err
	}
	registrar.Store(client.Id, client.IP)

	log.Printf("\t- Successfully stored entry in daemons: %+v\n", ipstr)
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

func deleteDaemon(daemon msgs.Client, registerErr error) {
	if registerErr != nil {
		return
	}

	ipstr := daemon.IP.String()
	deleted := daemons.CompareAndDelete(daemon.Id, ipstr)
	if deleted {
		log.Printf(
			"[INFO] Successfully deleted key-value pair from daemons\n\t- key: %v\n\t- value: %v\n\n",
			daemon.Id,
			ipstr)
	} else {
		log.Printf("[INFO] Delete failed, could not find key `%s`\n\b", daemon.Id)
	}
}
