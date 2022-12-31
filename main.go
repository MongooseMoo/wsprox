package main

import (
	"fmt"
	"net"
	"net/http"
	"strconv"

	"github.com/gorilla/websocket"
)

// constants for upstream and port
const upstream = "mongoose.moo.mud.org"
const upstreamPort = "7777"
const listenAddress = ":8080"

// IncomingWebsocketListener serves websocket connections from clients
func IncomingWebsocketListener(w http.ResponseWriter, r *http.Request) {
	// Upgrade the HTTP connection to a websocket connection
	conn, err := websocket.Upgrade(w, r, nil, 1024, 1024)
	if err != nil {
		fmt.Println(err)
		return
	}
	// Get the IP address of the client
	clientIP := conn.RemoteAddr().String()
	// Get the port of the client as a string
	clientPort := strconv.Itoa(conn.RemoteAddr().(*net.TCPAddr).Port)
	// IPv4 or IPv6
	isIPv4 := conn.RemoteAddr().(*net.TCPAddr).IP.To4() != nil
	// log the incoming connection
	fmt.Println("Incoming connection from " + clientIP + ":" + clientPort)
	tcpAddr, _ := net.ResolveTCPAddr("tcp", upstream+":"+upstreamPort)
	tcpConn, _ := net.DialTCP("tcp", nil, tcpAddr)
	// Send the client's connection info to the TCP server
	// Format: PROXY TCP4 source_addr dest_addr src_port dst_port
	// or TCP6 if ipv6
	if isIPv4 {
		_, err = tcpConn.Write([]byte("PROXY TCP4 " + clientIP + " " + listenAddress + " " + clientPort + " " + upstreamPort))
	} else {
		_, err = tcpConn.Write([]byte("PROXY TCP6 " + clientIP + " " + listenAddress + " " + clientPort + " " + upstreamPort))
	}

	if err != nil {
		fmt.Println(err)
		return
	}
	// Start proxying data from the websocket connection to the TCP connection
	// and vice versa
	go func() {
		for {
			// Read data from the websocket connection
			_, message, err := conn.ReadMessage()
			if err != nil {
				fmt.Println(err)
				break
			}
			// Write data to the TCP connection
			_, err = tcpConn.Write(message)
			if err != nil {
				fmt.Println(err)
				break
			}
		}
	}()
	go func() {
		for {
			// Read data from the TCP connection
			data := make([]byte, 1024)
			_, err := tcpConn.Read(data)
			if err != nil {
				fmt.Println(err)
				break
			}
			// Write data to the websocket connection
			err = conn.WriteMessage(websocket.TextMessage, data)
			if err != nil {
				fmt.Println(err)
				break
			}
		}
	}()
}

func main() {
	// Start the websocket server
	http.HandleFunc("/", IncomingWebsocketListener)
	fmt.Println("Listening on " + listenAddress)
	err := http.ListenAndServe(listenAddress, nil)
	if err != nil {
		fmt.Println(err)

	}

}
