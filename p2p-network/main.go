package main

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"strings"
)

const (
	defaultPort = "8080"
	bufferSize  = 1024
)

type Peer struct {
	ID   string
	Addr *net.TCPAddr
}

func main() {
	fmt.Println("Enter 'listen' to wait for connections or 'connect <address>' to connect to a peer:")
	input := readUserInput()

	if strings.HasPrefix(input, "connect ") {
		address := strings.TrimPrefix(input, "connect ")
		connectToPeer(address)
	} else if input == "listen" {
		startListener()
	} else {
		fmt.Println("Invalid input")
	}
}

func readUserInput() string {
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Scan()
	return scanner.Text()
}

func startListener() {
	peer := &Peer{
		ID: "Peer1",
		Addr: &net.TCPAddr{
			IP:   net.ParseIP("0.0.0.0"),
			Port: 8080,
		},
	}

	listener, err := net.ListenTCP("tcp", peer.Addr)
	if err != nil {
		log.Fatalf("Failed to start listener: %v", err)
	}
	defer listener.Close()

	fmt.Printf("Peer %s listening on %s\n", peer.ID, listener.Addr())

	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Printf("Failed to accept connection: %v", err)
			continue
		}
		go handleConnection(conn)
	}
}

func handleConnection(conn net.Conn) {
	defer conn.Close()
	fmt.Printf("New connection established with %s\n", conn.RemoteAddr())

	done := make(chan struct{})

	go receiveMessages(conn, done)
	go sendMessages(conn)

	<-done
}

func receiveMessages(conn net.Conn, done chan<- struct{}) {
	defer close(done)
	for {
		buffer := make([]byte, bufferSize)
		n, err := conn.Read(buffer)
		if err != nil {
			if err != io.EOF {
				fmt.Printf("Error reading from connection: %v\n", err)
			}
			fmt.Printf("Connection closed: %v\n", err)
			return
		}
		handleReceivedMessage(buffer[:n])
	}
}

func handleReceivedMessage(message []byte) {
	fmt.Printf("Received: %s", string(message))
	// Add any additional processing of the received message here
}

func sendMessages(conn net.Conn) {
	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Print("Enter message: ")
		if !scanner.Scan() {
			return
		}
		message := scanner.Bytes()
		if err := sendMessage(conn, message); err != nil {
			fmt.Printf("Failed to send message: %v\n", err)
			return
		}
	}
}

func sendMessage(conn net.Conn, message []byte) error {
	_, err := conn.Write(append(message, '\n'))
	return err
}

func connectToPeer(address string) {
	conn, err := net.Dial("tcp", address)
	if err != nil {
		log.Printf("Failed to connect to peer: %v", err)
		return
	}
	defer conn.Close()
	handleConnection(conn)
}
