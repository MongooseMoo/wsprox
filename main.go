package main

import (
	"fmt"
	"net"
	"net/http"

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
	// Connect to the TCP server
	// abstract this out to use a constant

	tcpAddr, _ := net.ResolveTCPAddr("tcp", upstream+":"+upstreamPort)
	tcpConn, _ := net.DialTCP("tcp", nil, tcpAddr)
	// Send the client's IP address to the TCP server
	_, err = tcpConn.Write([]byte(clientIP))
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
	http.ListenAndServe(listenAddress, nil)
}
