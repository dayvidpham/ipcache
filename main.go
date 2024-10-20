package main

import "fmt"
import "crypto/tls"

func main() {
	fmt.Println("Hello, World!")

	cert, err := tls.LoadX509KeyPair("./certs/ca.pem", "./certs/ca.pub")
	if err != nil {
		panic(err)
	}
	fmt.Println(cert)
}
