package main

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"strconv"

	"github.com/gorilla/websocket"
)

// constants for upstream and port
const upstream = "localhost"
const upstreamPort = "7778"
const listenAddress = "127.0.0.1:7654"
const bufferSize = 4096

func IncomingWebsocketListener(w http.ResponseWriter, r *http.Request) {
	log.Println("Received websocket connection request")

	// Upgrade the HTTP connection to a websocket connection
	conn, err := websocket.Upgrade(w, r, nil, bufferSize, bufferSize)
	if err != nil {
		log.Println("Error upgrading HTTP connection to websocket:", err)
		return
	}
	defer conn.Close()

	// Get the client IP and port
	clientIP := conn.RemoteAddr().(*net.TCPAddr).IP
	clientPort := conn.RemoteAddr().(*net.TCPAddr).Port
	// connection may be proxied, check headers
	if xForwardedFor := r.Header.Get("X-Forwarded-For"); xForwardedFor != "" {
		clientIP = net.ParseIP(xForwardedFor)
		// client port
		if xForwardedPort := r.Header.Get("X-Forwarded-Port"); xForwardedPort != "" {
			clientPort, err = strconv.Atoi(xForwardedPort)
			if err != nil {
				log.Println("Error parsing X-Forwarded-Port:", err)
				return
			}
		}
	}

	if clientIP == nil {
		log.Println("Error getting client IP address")
		return
	}
	// Connect to the upstream server
	tcpAddr, err := net.ResolveTCPAddr("tcp", upstream+":"+upstreamPort)
	if err != nil {
		log.Println("Error resolving upstream address:", err)
		return
	}
	// Build the PROXY line to send to the upstream server
	proxyLine := "PROXY TCP4"
	if clientIP.To4() == nil {
		proxyLine = "PROXY TCP6"
	}
	proxyLine += fmt.Sprintf(" %s %s %d %s\n", clientIP, tcpAddr.IP, clientPort, upstreamPort)
	log.Printf("Sending PROXY line to upstream server: %s\n", proxyLine)

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
			data := make([]byte, bufferSize)
			n, err := tcpConn.Read(data)
			if err != nil {
				errc <- err
				return
			}
			err = conn.WriteMessage(websocket.BinaryMessage, data[:n])
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
