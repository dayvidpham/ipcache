package main

import (
	"crypto/tls"
	"log"
	//"net"
	//"bufio"
)

func main() {
	// Referencing https://gist.github.com/denji/12b3a568f092ab951456
	log.SetFlags(log.Lshortfile)
	config := &tls.Config{
		InsecureSkipVerify: true,
	}

	conn, err := tls.Dial("tcp", "127.0.0.1:4430", config)
	if err != nil {
		log.Println(err)
		return
	}
	defer conn.Close()

	n, err := conn.Write([]byte("Hello from Client\n"))
	if err != nil {
		log.Println(n, err)
		return
	}

	buf := make([]byte, 100)
	n, err = conn.Read(buf)
	if err != nil {
		log.Println(n, err)
		return
	}

	log.Println("Received from server:", string(buf[:n]))
}
