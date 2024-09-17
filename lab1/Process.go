package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"strconv"
	"time"
)

type Message struct {
	Code     int
	Text     string
	MsgClock int
}

// Global variables
var id int
var clock int
var myPort string          // Port the process listens on
var nServers int           // Number of other processes
var CliConn []*net.UDPConn // Connections to other processes
var ServConn *net.UDPConn  // Server connection to receive messages

func CheckError(err error) {
	if err != nil {
		fmt.Println("Error:", err)
		os.Exit(0)
	}
}

func PrintError(err error) {
	if err != nil {
		fmt.Println("Error:", err)
	}
}

func doServerJob() {
	buf := make([]byte, 1024)
	// Infinite loop
	for {
		// Read a message from the UDP connection
		n, addr, err := ServConn.ReadFromUDP(buf)
		PrintError(err)

		var msg Message
		err = json.Unmarshal(buf[:n], &msg)
		PrintError(err)

		// Print the received message along with the sender's address
		fmt.Println("Received", msg.Code, "-", msg.Text, "with MsgClock", msg.MsgClock, "from", addr)
	}
}

func doClientJob(otherProcess int) {
	msg := Message{Code: 123, Text: "teste", MsgClock: clock}

	jsonMsg, err := json.Marshal(msg)
	PrintError(err)

	_, err = CliConn[otherProcess].Write(jsonMsg)
	PrintError(err)
}

func initConnections() {
	var err error
	id, err = strconv.Atoi(os.Args[1])
	CheckError(err)
	clock = 0

	// Total number of processes (including self)
	nTotalServers := len(os.Args) - 2

	// Our own port is at position id in the list (since id starts from 1)
	myPort = os.Args[1+id]

	// Number of other servers
	nServers = nTotalServers - 1

	// Initialize connections to other servers
	CliConn = make([]*net.UDPConn, nServers)

	// Set up the server connection to receive messages
	ServerAddr, err := net.ResolveUDPAddr("udp", "127.0.0.1"+myPort)
	CheckError(err)
	ServConn, err = net.ListenUDP("udp", ServerAddr)
	CheckError(err)

	// Set up connections to other servers
	sIdx := 0
	for s := 1; s <= nTotalServers; s++ {
		if s == id {
			// Skip our own process
			continue
		}
		ServerAddr, err := net.ResolveUDPAddr("udp", "127.0.0.1"+os.Args[1+s])
		CheckError(err)
		Conn, err := net.DialUDP("udp", nil, ServerAddr)
		CheckError(err)
		CliConn[sIdx] = Conn
		sIdx++
	}
}

func readInput(ch chan string) {
	// Routine to listen to stdin
	reader := bufio.NewReader(os.Stdin)
	for {
		text, _, err := reader.ReadLine()
		if err != nil {
			fmt.Println("Error reading input:", err)
			continue
		}
		ch <- string(text)
	}
}

func main() {
	initConnections()

	ch := make(chan string) // Channel to store keyboard input
	go readInput(ch)        // Start routine to listen to keyboard
	go doServerJob()

	for {
		// Non-blocking check for stdin input
		select {
		case x, valid := <-ch:
			if valid {
				fmt.Printf("From keyboard: %s\n", x)
				for j := 0; j < nServers; j++ {
					go doClientJob(j)
				}
			} else {
				fmt.Println("Closed channel!")
			}
		default:
			// Do nothing but avoid blocking
			time.Sleep(time.Second * 1)
		}
		// Wait a bit
		time.Sleep(time.Second * 1)
	}
}
