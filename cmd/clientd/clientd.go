package main

import (
	"crypto/tls"
	"crypto/x509"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

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

var (
	parsedServer                 string
	parsedPort                   uint
	parsedCertPath               string
	parsedPrivatekeyPath         string
	parsedServerRootCACert       string
	parsedRegisterTimeoutSeconds uint
	nflagsRequired               int = 5

	registerTimeout time.Duration
)

var (
	pingTimeout   time.Duration
	sleepDuration time.Duration
)

func init() {
	flag.StringVar(&parsedServer, "server", "", "server to connect to; examples <ipcache.com | 192.168.0.1> ")
	flag.UintVar(&parsedPort, "port", 0, "server port to connect to; examples <8080 | 4430>")
	flag.StringVar(&parsedCertPath, "cert", "", "path to your certificate")
	flag.StringVar(&parsedPrivatekeyPath, "privatekey", "", "path to your private key that was used to sign your certificate")
	flag.StringVar(&parsedServerRootCACert, "server-root-ca-cert", "", "path to the expected server root CA certificate, used for verification")
	flag.UintVar(&parsedRegisterTimeoutSeconds, "register-timeout-seconds", 10, "max time to wait for server to respond to ClientRegister message before killing the connection")
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
	log.Println("[DEBUG] --register-timeout-seconds", parsedRegisterTimeoutSeconds)

	// NOTE: Want some data type binding (var, flagname, Flag) for convenience error-checking
	// Could also maybe use the Visitor for error-checking?
	if flag.NFlag() < nflagsRequired {
		log.Panicf("[FATAL] Require at least %d flags to be set, but were only given %d\n", nflagsRequired, flag.NFlag())
	}

	parsedServerAddr := fmt.Sprintf("%s:%d", parsedServer, parsedPort)
	registerTimeout = time.Second * time.Duration(parsedRegisterTimeoutSeconds)

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

	client := msgs.NewMessenger(conn)

	/*
		Register with server, kill connection if server response takes too long
		Should receive the expected ping timeout from the server as a response
	*/
	sendMsg := msgs.ClientRegister()
	err = client.Send(sendMsg)
	log.Printf("Sending ClientRegister message\n")
	if err != nil {
		log.Println(err)
		return
	}
	client.SetReadTimeout(registerTimeout)

	timeoutMsg, err := client.Receive()
	if err != nil {
		log.Println(err)
		return
	}
	if timeoutMsg.Type() != msgs.T_String {
		log.Printf("[FATAL] Expected the server to respond with MessageType String, but got %s.\n", timeoutMsg.Type())
	}

	log.Printf(
		"Registration succeeded.\n\t- Got ping timeout from server, %d total bytes\n\t- Ping timeout: %s\n\n",
		timeoutMsg.Size(),
		timeoutMsg.Payload())

	// Unset the register timeout
	err = client.SetReadDeadline(time.Time{})
	if err != nil {
		log.Println(err)
		return
	}

	pingTimeout, err = time.ParseDuration(string(timeoutMsg.Payload()))
	if err != nil {
		log.Printf("[FATAL] Failed to parse server's response payload as a time.Duration.\n\t- Received: %s\n", timeoutMsg.Payload())
		return
	}
	sleepDuration = time.Duration((pingTimeout * 3) / 4)
	log.Printf(
		"[INFO] Calculated ping interval as timeout * 3/4\n\t- (%s) * 3/4 = %s\n\n",
		pingTimeout,
		sleepDuration)
	// Registration complete

	for {
		err = client.SetWriteTimeout(pingTimeout)
		if err != nil {
			log.Println(err)
			return
		}
		sendMsg = msgs.Ping()

		err := client.Send(sendMsg)
		if err != nil {
			log.Println(err)
			return
		}

		log.Printf("Sent Ping to server. Sleeping for %v seconds.\n", sleepDuration)
		time.Sleep(sleepDuration)
	}

}
