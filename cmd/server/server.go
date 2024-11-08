package main

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"time"

	"github.com/dayvidpham/ipcache/internal/msgs"
	//"database/sql"
	//"github.com/mattn/go-sqlite3"
)

func main() {
	// Referencing https://gist.github.com/denji/12b3a568f092ab951456
	log.SetFlags(log.Lshortfile)

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
		log.Println("[INFO] New tls.Conn established with", conn.RemoteAddr().String(), ", still need to do TLS handshake")

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
	err = conn.SetDeadline(time.Now().Add(time.Second * 3))
	if err != nil {
		log.Println("[ERROR] Failed to set a deadline for this tls.Conn, rejecting conn\n\t", err)
		return
	}

	// NOTE: The TLS handshake is normally performed lazily, but do eagerly to fail fast
	if err = conn.Handshake(); err != nil {
		log.Println("[ERROR] Failed TLS handshake for", conn.RemoteAddr().String(), ".\n\t- Reason:", err)
		return
	}

	if client, err = msgs.NewClient(conn); err != nil {
		log.Println(err)
		return
	}
	log.Println("[INFO] TLS handshake succeeded!")
	conn.SetDeadline(time.Time{})

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

		err = nil
		switch recvMsg.Type() {
		case msgs.T_ClientRegister:
			err = ClientRegisterHandler(server, recvMsg, client)
		case msgs.T_String:
			StringMessageHandler(recvMsg, client)
		case msgs.T_Ping:
			log.Printf("[INFO] Received from %+v: %s\n\tAttempt to SetReadTimeout().\n\n", client, recvMsg.Type())
			err = server.SetReadTimeout(time.Second * 20)
		default:
			err = fmt.Errorf("[ERROR] Unimplemented message type:\n\t%s\n", recvMsg.Type())
		}
		if err != nil {
			log.Println(err)
			return
		}
	}
}

func StringMessageHandler(recvMsg msgs.Message, client msgs.Client) {
	log.Printf("Received from %+v: %s\n\tPayload: %s\n", client, recvMsg.Type(), recvMsg.Payload())
}
func ClientRegisterHandler(server msgs.Messenger, recvMsg msgs.Message, client msgs.Client) (err error) {
	log.Printf("Received from %+v: %s\n\tResponding with Ok ...\n", client, recvMsg.Type())
	if err = server.Send(msgs.Ok()); err != nil {
		return fmt.Errorf("[ERROR] Failed to Send Ok\n\t%w\n", err)
	}
	log.Println("\t[INFO] Sent Ok. Proceeding to SetReadTimeout.")

	err = server.SetReadTimeout(time.Second * 20)
	if err != nil {
		return fmt.Errorf("[ERROR] Failed during SetReadTimeout\n\t%w\n", err)
	}

	return err
}
