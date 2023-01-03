package main

import (
	"fmt"
	"log"
	"net"
	"net/http"

	"github.com/gorilla/websocket"
)

// constants for upstream and port
const upstream = "mongoose.moo.mud.org"
const upstreamPort = "7777"
const listenAddress = ":7654"

func IncomingWebsocketListener(w http.ResponseWriter, r *http.Request) {
	log.Println("Received websocket connection request")

	// Upgrade the HTTP connection to a websocket connection
	conn, err := websocket.Upgrade(w, r, nil, 1024, 1024)
	if err != nil {
		log.Println("Error upgrading HTTP connection to websocket:", err)
		return
	}
	defer conn.Close()

	// Get the client IP and port
	clientIP := conn.RemoteAddr().(*net.TCPAddr).IP
	clientPort := conn.RemoteAddr().(*net.TCPAddr).Port

	// Build the PROXY line to send to the upstream server
	proxyLine := "PROXY TCP4"
	if clientIP.To4() == nil {
		proxyLine = "PROXY TCP6"
	}
	proxyLine += fmt.Sprintf(" %s %s %d %s\r\n", clientIP, listenAddress, clientPort, upstreamPort)
	log.Printf("Sending PROXY line to upstream server: %s\n", proxyLine)

	// Connect to the upstream server
	tcpAddr, err := net.ResolveTCPAddr("tcp", upstream+":"+upstreamPort)
	if err != nil {
		log.Println("Error resolving upstream address:", err)
		return
	}

	tcpConn, err := net.DialTCP("tcp", nil, tcpAddr)
	if err != nil {
		log.Println("Error connecting to upstream server:", err)
		return
	}
	defer tcpConn.Close()

	// Send the PROXY line to the upstream server
	_, err = tcpConn.Write([]byte(proxyLine))
	if err != nil {
		log.Println("Error sending PROXY line to upstream server:", err)
		return
	}

	// Create a channel to receive errors from the goroutines
	errc := make(chan error, 2)

	// Start a goroutine to forward data from the websocket connection to the upstream connection
	go func() {
		for {
			_, message, err := conn.ReadMessage()
			if err != nil {
				errc <- err
				return
			}
			_, err = tcpConn.Write(message)
			if err != nil {
				errc <- err
				return
			}
		}
	}()

	// Start a goroutine to forward data from the upstream connection to the websocket connection
	go func() {
		for {
			data := make([]byte, 1024)
			_, err := tcpConn.Read(data)
			if err != nil {
				errc <- err
				return
			}
			err = conn.WriteMessage(websocket.BinaryMessage, data)
			if err != nil {
				errc <- err
				return
			}
		}
	}()

	// Wait for an error from one of the goroutines
	select {
	case err := <-errc:
		log.Println("Error forwarding data:", err)

	}
}

func main() {
	// Start the websocket server
	http.HandleFunc("/", IncomingWebsocketListener)
	log.Println("Listening on " + listenAddress)
	err := http.ListenAndServe(listenAddress, nil)
	if err != nil {
		log.Println("Error starting websocket server:", err)
	}
}
