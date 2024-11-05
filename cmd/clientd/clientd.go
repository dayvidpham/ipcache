package main

import (
	"bufio"
	"crypto/tls"
	"crypto/x509"
	"encoding/gob"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/dayvidpham/ipcache/internal/msgs"
)

//////////////////////////////////////////////////////////////
// Declare and init basic CLI flags
//////////////////////////////////////////////////////////////

/**
 * Required flags:
 * --server <hostname | IP (probably just tls.Dial format)>
 * --port <positive number>
 * --cert <path>
 * --privatekey <path>
 * --publickey <path> (probably don't need)
 *
 * --server root CAs
 */

var parsedServer string
var parsedPort uint
var parsedCertPath string
var parsedPrivatekeyPath string
var parsedServerRootCACert string
var nflagsRequired int = 5

func init() {
	flag.StringVar(&parsedServer, "server", "", "server to connect to; examples <ipcache.com | 192.168.0.1> ")
	flag.UintVar(&parsedPort, "port", 0, "server port to connect to; examples <8080 | 4430>")
	flag.StringVar(&parsedCertPath, "cert", "", "path to your certificate")
	flag.StringVar(&parsedPrivatekeyPath, "privatekey", "", "path to your private key that was used to sign your certificate")
	flag.StringVar(&parsedServerRootCACert, "server-root-ca-cert", "", "path to the expected server root CA certificate, used for verification")
}

func main() {
	log.SetFlags(log.Lshortfile)

	///////////////////////////////
	// Proccess flags
	///////////////////////////////
	flag.Parse()
	log.Println("[DEBUG] --server", parsedServer)
	log.Println("[DEBUG] --port", parsedPort)
	log.Println("[DEBUG] --cert", parsedCertPath)
	log.Println("[DEBUG] --privatekey", parsedPrivatekeyPath)
	log.Println("[DEBUG] --server-root-ca-cert", parsedServerRootCACert)

	// NOTE: Want some data type binding (var, flagname, Flag) for convenience error-checking
	// Could also maybe use the Visitor for error-checking?
	if flag.NFlag() < nflagsRequired {
		log.Panicf("[FATAL] Require at least %d flags to be set, but were only given %d\n", nflagsRequired, flag.NFlag())
	}

	parsedServerAddr := fmt.Sprintf("%s:%d", parsedServer, parsedPort)

	///////////////////////////////
	// Main client daemon program
	// Referencing https://gist.github.com/denji/12b3a568f092ab951456
	///////////////////////////////

	caCert, err := os.ReadFile(parsedServerRootCACert)
	if err != nil {
		log.Println("[FATAL] Reading Root CA PEM file failed.\n\t- Reason:", err)
		return
	}

	caCertPool := x509.NewCertPool()
	caCertPool.AppendCertsFromPEM(caCert)

	cert, err := tls.LoadX509KeyPair(parsedCertPath, parsedPrivatekeyPath)
	if err != nil {
		log.Println("[FATAL] Loading X509 key pair failed.\n\t- Reason:", err)
		return
	}

	config := tls.Config{
		MinVersion:   tls.VersionTLS13,
		Certificates: []tls.Certificate{cert},
		RootCAs:      caCertPool,
	}

	conn, err := tls.Dial("tcp", parsedServerAddr, &config)
	if err != nil {
		log.Println("[FATAL] Failed to establish connection to the server at", parsedServerAddr, "\n\t- Reason:", err)
		return
	}
	defer conn.Close()

	///////////////////////////////
	// Handle responses from server
	// Referencing https://gist.github.com/denji/12b3a568f092ab951456
	///////////////////////////////

	r, w := bufio.NewReader(conn), bufio.NewWriter(conn)
	rw := bufio.NewReadWriter(r, w)
	enc, dec := gob.NewEncoder(rw), gob.NewDecoder(rw)

	//n, err := rw.Write([]byte("Hello from Client\n"))
	msgreg := msgs.ClientRegister()
	err = enc.Encode(&msgreg)
	if err != nil {
		log.Panicln("[FATAL]: Failed to encode ClientRegisterMessage\n\t- Reason:", err)
	}
	rw.Flush()

	var msgOk msgs.Message
	err = dec.Decode(&msgOk)
	if err != nil {
		log.Println(err)
		return
	}
	log.Println("[INFO] Received from server", msgOk)

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
