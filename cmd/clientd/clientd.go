package main

import (
	"bufio"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"log"
	"strings"

	//"net"
	"os"
)

func main() {
	/**
	 * Required flags:
	 * --server <hostname | IP (probably just tls.Dial format)>
	 * --cert
	 * --privatekey?
	 * --publickey (probably don't need)
	 */

	// Referencing https://gist.github.com/denji/12b3a568f092ab951456
	log.SetFlags(log.Lshortfile)

	caCert, err := os.ReadFile("certs/self.pem")
	if err != nil {
		log.Println("[FATAL]: Reading Root CA PEM file failed.\n\t- Reason:", err)
		return
	}

	caCertPool := x509.NewCertPool()
	caCertPool.AppendCertsFromPEM(caCert)
	
	cert, err := tls.LoadX509KeyPair("certs/self.pem", "certs/self.key")
	if err != nil {
		log.Println("[FATAL]: Loading X509 key pair failed.\n\t- Reason:", err)
		return
	}

	config := tls.Config{
		RootCAs: caCertPool,
		Certificates: []tls.Certificate{cert},
	}

	log.Println("Sending", len(config.Certificates), "certificates")
	//log.Println("- the cert:", config.Certificates[0])

	/*
	 * // From https://smallstep.com/hello-mtls/doc/combined/go/go
	 * caCert, _ := ioutil.ReadFile("ca.crt")
	 * caCertPool := x509.NewCertPool()
	 * caCertPool.AppendCertsFromPEM(caCert)
	 * 
	 * cert, _ := tls.LoadX509KeyPair("client.crt", "client.key")
	 * 
	 * client := &http.Client{
	 *     Transport: &http.Transport{
	 *         TLSClientConfig: &tls.Config{
	 *             RootCAs: caCertPool,
	 *             Certificates: []tls.Certificate{cert},
	 *         },
	 *     },
	 * }
	 * // Make a request
	 * r, err := client.Get("https://myserver.internal.net:9443")
	 */

	conn, err := tls.Dial("tcp", "127.0.0.1:4430", &config)
	if err != nil {
		log.Println(err)
		return
	}
	defer conn.Close()

	log.Printf("%+v\n", conn.ConnectionState())

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
