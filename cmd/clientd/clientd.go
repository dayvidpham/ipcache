package main

import (
	"bufio"
	"crypto/tls"
	"fmt"
	"log"
	"strings"

	//"net"
	"os"
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

	r, w := bufio.NewReader(conn), bufio.NewWriter(conn)
	rw := bufio.NewReadWriter(r, w)
	n, err := rw.Write([]byte("Hello from Client\n"))
	if err != nil {
		log.Println(n, err)
		return
	}

	//buf := make([]byte, 100)
	str, err := rw.ReadString('\n')
	if err != nil {
		log.Println(n, err)
		return
	}
	log.Println("Received from server:", str)

	scan := bufio.NewScanner(os.Stdin)
	fmt.Print(">>> ")

	for scan.Scan() {
		input := strings.TrimSpace(scan.Text())
		if len(input) == 0 {
			fmt.Print(">>> ")
			continue
		}

		n, err := rw.Write([]byte(input + "\n"))
		log.Println("Wrote", n, "bytes")
		if err != nil {
			log.Println(err)
			return
		}

		err = rw.Flush()
		if err != nil {
			log.Println(err)
			return
		}

		fmt.Print(">>> ")
	}

}
