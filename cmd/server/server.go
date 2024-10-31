package main

import "net"
import "crypto/tls"
import "log"
import "bufio"

func main() {
	// Referencing https://gist.github.com/denji/12b3a568f092ab951456
	log.SetFlags(log.Lshortfile)
	log.Println("Hello, World!")

	cert, err := tls.LoadX509KeyPair("./certs/desktop.pem", "./certs/desktop.key")
	if err != nil {
		panic(err)
	}

	config := tls.Config{Certificates: []tls.Certificate{cert}}
	ln, err := tls.Listen("tcp", ":4430", &config)
	if err != nil {
		panic(err)
	}
	defer ln.Close()

	for {
		conn, err := ln.Accept()
		if err != nil {
			log.Println("Failed to accept connection\n\tGot error: ", err)
			continue
		}
		go tlsServe(conn)
	}

	return
}

func tlsServe(conn net.Conn) {
	defer conn.Close()

	r := bufio.NewReader(conn)
	for {
		msg, err := r.ReadString('\n')
		if err != nil {
			log.Println("Call to ReadString failed\n\tGot error: ", err)
			return
		}
		log.Println(msg)
		
		n, err := conn.Write([]byte("World\n"))
		log.Println("Wrote ", n, "bytes")
		if err != nil {
			log.Println("\tGot error: ", err)
			return
		}
	}

	return
}
