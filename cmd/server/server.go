package main

import (
	"bufio"
	"crypto/tls"
	"io"
	"log"
	"net"
)

func main() {
	// Referencing https://gist.github.com/denji/12b3a568f092ab951456
	log.SetFlags(log.Lshortfile)
	log.Println("Hello, World!")

	cert, err := tls.LoadX509KeyPair("./certs/self.pem", "./certs/self.key")
	if err != nil {
		panic(err)
	}

	config := &tls.Config{Certificates: []tls.Certificate{cert}}
	ln, err := tls.Listen("tcp", ":4430", config)
	if err != nil {
		panic(err)
	}
	defer ln.Close()

	for {
		conn, err := ln.Accept()
		if err != nil {
			log.Println("Failed to accept connection\n\tGot error:", err)
			continue
		}
		go tlsServe(conn)
	}

}

func tlsServe(conn net.Conn) {
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
