package main

import (
	"fmt"
	"net"
	"os"
	"strconv"
	"time"
)

var (
	myAddress string         // Endereço do meu servidor (IP:porta)
	nServers  int            // Quantidade de outros processos
	CliConn   []*net.UDPConn // Vetor com conexões para os servidores dos outros processos
	ServConn  *net.UDPConn   // Conexão do meu servidor (onde recebo mensagens dos outros processos)
)

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
	for {
		n, addr, err := ServConn.ReadFromUDP(buf)
		PrintError(err)

		fmt.Println("Received", string(buf[0:n]), "from", addr)
	}
}

func doClientJob(otherProcess int, i int) {
	msg := strconv.Itoa(i)
	i++
	buf := []byte(msg)
	_, err := CliConn[otherProcess].Write(buf)
	PrintError(err)
}

func initConnections() {
	myAddress = os.Args[1]
	nServers = len(os.Args) - 2
	CliConn = make([]*net.UDPConn, nServers)

	ServerAddr, err := net.ResolveUDPAddr("udp", myAddress)
	CheckError(err)
	ServConn, err = net.ListenUDP("udp", ServerAddr)
	CheckError(err)

	for s := 0; s < nServers; s++ {
		ServerAddr, err := net.ResolveUDPAddr("udp", os.Args[2+s])
		CheckError(err)
		Conn, err := net.DialUDP("udp", nil, ServerAddr)
		CliConn[s] = Conn
		CheckError(err)
	}
}

func main() {
	initConnections()

	defer ServConn.Close()
	for i := 0; i < nServers; i++ {
		defer CliConn[i].Close()
	}

	go doServerJob()
	i := 0
	for {
		for j := 0; j < nServers; j++ {
			go doClientJob(j, i)
		}
		time.Sleep(time.Second * 1)
		i++
	}
}
