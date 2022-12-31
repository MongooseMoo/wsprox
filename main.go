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
const listenAddress = ":8080"

func IncomingWebsocketListener(w http.ResponseWriter, r *http.Request) {
	conn, err := websocket.Upgrade(w, r, nil, 1024, 1024)
	if err != nil {
		log.Println(err)
		return
	}
	defer conn.Close()

	clientIP := conn.RemoteAddr().(*net.TCPAddr).IP
	clientPort := conn.RemoteAddr().(*net.TCPAddr).Port
	proxyLine := "PROXY TCP4"
	if clientIP.To4() == nil {
		proxyLine = "PROXY TCP6"
	}
	proxyLine += fmt.Sprintf(" %s %s %d %s", clientIP, listenAddress, clientPort, upstreamPort)

	tcpAddr, err := net.ResolveTCPAddr("tcp", upstream+":"+upstreamPort)
	if err != nil {
		log.Println(err)
		return
	}

	tcpConn, err := net.DialTCP("tcp", nil, tcpAddr)
	if err != nil {
		log.Println(err)
		return
	}
	defer tcpConn.Close()

	_, err = tcpConn.Write([]byte(proxyLine))
	if err != nil {
		log.Println(err)
		return
	}

	errc := make(chan error, 2)
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
	go func() {
		for {
			data := make([]byte, 1024)
			_, err := tcpConn.Read(data)
			if err != nil {
				errc <- err
				return
			}
			err = conn.WriteMessage(websocket.TextMessage, data)
			if err != nil {
				errc <- err
				return
			}
		}
	}()

	select {
	case err := <-errc:
		log.Println(err)
	}
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
