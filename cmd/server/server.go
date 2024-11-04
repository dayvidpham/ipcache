package main

import (
	"bufio"
	"crypto/tls"
	"crypto/x509"
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

	config := &tls.Config{
		Certificates: []tls.Certificate{cert},
		ClientCAs: caCertPool,
		ClientAuth: tls.RequireAndVerifyClientCert,
	}
	ln, err := tls.Listen("tcp", ":4430", config)
	if err != nil {
		panic(err)
	}
	defer ln.Close()

	for {
		netconn, err := ln.Accept()
		if err != nil {
			log.Println("Failed to accept connection\n\tGot error:", err)
			continue
		}

		conn, ok := netconn.(*tls.Conn)
		if !ok {
			log.Println("[ERROR]: Connection establed but is not a TLS connection for some reason.")
			netconn.Close()
			continue
		}
		log.Println("[INFO]: New TLS connection established!")

		connstate := conn.ConnectionState()
		clientcerts := connstate.VerifiedChains
		log.Printf("\tClient provided %d certificates\n", len(clientcerts))
		log.Println()
		log.Printf("%+v\n", connstate)
		//log.Println("\tleaf cert:", clientcerts[0])

		go tlsServe(conn)
	}

}

func tlsServe(conn *tls.Conn) {
	defer conn.Close()

	r, w := bufio.NewReader(conn), bufio.NewWriter(conn)
	rw := bufio.NewReadWriter(r, w)

	n, err := rw.Write([]byte("Hello from server\n"))
	if err != nil {
		log.Println("\tGot error:", err)
		return
	}
	err = rw.Flush()
	if err != nil {
		log.Println("\tGot error:", err)
		return
	}
	log.Println("Wrote", n, "bytes")

	for {
		msg, err := rw.ReadString('\n')
		switch err {
		case nil:
			break
		case io.EOF:
			log.Println("Connection closed")
			return
		default:
			log.Println("Call to ReadString failed\n\tGot error: ", err)
			return
		}
		log.Print("Received from client:", msg)
	}

}
