package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"strconv"

	"github.com/gorilla/websocket"
)

// Config holds the configuration settings for the server
type Config struct {
	Upstream      string
	UpstreamPort  string
	ListenAddress string
	BufferSize    int
}

func main() {
	config := parseConfig()
	setupLogger()

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		incomingWebsocketListener(w, r, config)
	})

	log.Printf("Listening on %s with buffer size %d\n", config.ListenAddress, config.BufferSize)
	if err := http.ListenAndServe(config.ListenAddress, nil); err != nil {
		log.Fatalf("Failed to start websocket server: %v\n", err)
	}
}

func parseConfig() Config {
	upstream := flag.String("upstream", getEnv("WSPROX_UPSTREAM", "localhost"), "Upstream server address")
	if *upstream == "" {
		log.Fatal("Upstream server address must be specified")
	}
	upstreamPort := flag.String("upstream-port", getEnv("WSPROX_UPSTREAM_PORT", "7778"), "Upstream server port")
	listenAddress := flag.String("listen-address", getEnv("WSPROX_LISTEN_ADDRESS", "127.0.0.1:7654"), "Address to listen on")
	bufferSize := flag.Int("buffer-size", getEnvAsInt("WSPROX_BUFFER_SIZE", 4096), "Size of the buffer in bytes")

	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "Usage of %s:\n", os.Args[0])
		flag.PrintDefaults()
	}
	flag.Parse()

	return Config{
		Upstream:      *upstream,
		UpstreamPort:  *upstreamPort,
		ListenAddress: *listenAddress,
		BufferSize:    *bufferSize,
	}
}

func getEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return fallback
}

func getEnvAsInt(key string, fallback int) int {
	if value, exists := os.LookupEnv(key); exists {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return fallback
}

func setupLogger() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
}

func incomingWebsocketListener(w http.ResponseWriter, r *http.Request, config Config) {
	log.Println("Received websocket connection request")

	conn, err := websocket.Upgrade(w, r, nil, config.BufferSize, config.BufferSize)
	if err != nil {
		log.Printf("Error upgrading HTTP to websocket: %v\n", err)
		return
	}
	defer conn.Close()

	clientIP, clientPort, err := processClientAddress(conn, r)
	if err != nil {
		log.Printf("Error processing client address: %v\n", err)
		return
	}

	tcpAddr, err := net.ResolveTCPAddr("tcp", fmt.Sprintf("%s:%s", config.Upstream, config.UpstreamPort))
	if err != nil {
		log.Printf("Error resolving upstream address: %v\n", err)
		return
	}

	proxyLine, err := buildProxyLine(clientIP, clientPort, tcpAddr, config.UpstreamPort)
	if err != nil {
		log.Printf("Error building proxy line: %v\n", err)
		return
	}

	tcpConn, err := net.DialTCP("tcp", nil, tcpAddr)
	if err != nil {
		log.Printf("Error connecting to upstream server: %v\n", err)
		return
	}
	defer tcpConn.Close()

	if _, err = tcpConn.Write([]byte(proxyLine)); err != nil {
		log.Printf("Error sending PROXY line to upstream server: %v\n", err)
		return
	}

	errc := make(chan error, 2)

	go forwardWebsocketToTCP(conn, tcpConn, errc, config.BufferSize)
	go forwardTCPToWebsocket(conn, tcpConn, errc, config.BufferSize)

	if err := <-errc; err != nil {
		log.Printf("Error forwarding data: %v\n", err)
	}
}

func processClientAddress(conn *websocket.Conn, r *http.Request) (net.IP, int, error) {
	clientIP := conn.RemoteAddr().(*net.TCPAddr).IP
	clientPort := conn.RemoteAddr().(*net.TCPAddr).Port

	if xForwardedFor := r.Header.Get("X-Forwarded-For"); xForwardedFor != "" {
		clientIP = net.ParseIP(xForwardedFor)
		if xForwardedPort := r.Header.Get("X-Forwarded-Port"); xForwardedPort != "" {
			var err error
			clientPort, err = strconv.Atoi(xForwardedPort)
			if err != nil {
				return nil, 0, fmt.Errorf("invalid X-Forwarded-Port value: %v", err)
			}
		}
	}

	if clientIP == nil {
		return nil, 0, fmt.Errorf("failed to obtain client IP address")
	}
	return clientIP, clientPort, nil
}

func buildProxyLine(clientIP net.IP, clientPort int, tcpAddr *net.TCPAddr, upstreamPort string) (string, error) {
	if clientIP == nil || tcpAddr == nil {
		return "", fmt.Errorf("invalid IP addresses")
	}

	proxyLine := "PROXY TCP4"
	if clientIP.To4() == nil {
		proxyLine = "PROXY TCP6"
	}
	return fmt.Sprintf("%s %s %s %d %s\n", proxyLine, clientIP, tcpAddr.IP, clientPort, upstreamPort), nil
}

func forwardWebsocketToTCP(conn *websocket.Conn, tcpConn *net.TCPConn, errc chan<- error, bufferSize int) {
	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			errc <- fmt.Errorf("websocket read error: %v", err)
			return
		}
		if _, err = tcpConn.Write(message); err != nil {
			errc <- fmt.Errorf("TCP write error: %v", err)
			return
		}
	}
}

func forwardTCPToWebsocket(conn *websocket.Conn, tcpConn *net.TCPConn, errc chan<- error, bufferSize int) {
	for {
		data := make([]byte, bufferSize)
		n, err := tcpConn.Read(data)
		if err != nil {
			errc <- fmt.Errorf("TCP read error: %v", err)
			return
		}
		if err = conn.WriteMessage(websocket.BinaryMessage, data[:n]); err != nil {
			errc <- fmt.Errorf("websocket write error: %v", err)
			return
		}
	}
}
