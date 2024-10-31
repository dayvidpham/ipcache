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

	}

	conn, err := tls.Dial("tcp", "localhost:4430", config)
	if err != nil {
		log.Println(err)
		return
	}
	defer conn.Close()

	n, err := conn.Write([]byte("hello\n"))
	if err != nil {
		log.Println(n, err)
		return
	}

}
