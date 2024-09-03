package main

import (
	"fmt"
	"net"
	"os"
	"time"
	"bufio"
	"encoding/json"
)

type Message struct {
	Code int
	Text string
}

// Variáveis globais interessantes para o processo
var err string
var myPort string        // porta do meu servidor
var nServers int         // quantidade de outros processos
var CliConn []*net.UDPConn // vetor com conexões para os servidores dos outros processos
var ServConn *net.UDPConn  // conexão do meu servidor (onde recebo mensagens dos outros processos)

func CheckError(err error) {
	if err != nil {
		fmt.Println("Error: ", err)
		os.Exit(0)
	}
}

func PrintError(err error) {
	if err != nil {
		fmt.Println("Error: ", err)
	}
}

func doServerJob() {
    buf := make([]byte, 1024)
    // Loop infinito mesmo
    for {
        // Ler (uma vez somente) da conexão UDP a mensagem
        n, addr, err := ServConn.ReadFromUDP(buf)
        PrintError(err)
        
        var msg Message
        err = json.Unmarshal(buf[:n], &msg)
        PrintError(err)
        
        // Escrever na tela a mensagem recebida (indicando o endereço de quem enviou)
        fmt.Println("Received", msg.Code, "-", msg.Text, "from", addr)
    }
}


func doClientJob(otherProcess int, i int) {
	msg := Message{Code: 123, Text: "teste"}
	
	jsonMsg, err := json.Marshal(msg)
	PrintError(err)
	
	_, err = CliConn[otherProcess].Write(jsonMsg)
	PrintError(err)
}

func initConnections() {
	myPort = os.Args[1]
	nServers = len(os.Args) - 2
	/* Esse 2 tira o nome (no caso Process) e tira a primeira porta (que é a minha).
	   As demais portas são dos outros processos */
	CliConn = make([]*net.UDPConn, nServers)

	// Configura a conexão do meu servidor (onde recebo mensagens).
	ServerAddr, err := net.ResolveUDPAddr("udp", "127.0.0.1"+myPort)
	CheckError(err)
	ServConn, err = net.ListenUDP("udp", ServerAddr)
	CheckError(err)

	// Configura a conexão com cada servidor dos outros processos e coloca as conexões no vetor CliConn.
	for s := 0; s < nServers; s++ {
		ServerAddr, err := net.ResolveUDPAddr("udp", "127.0.0.1"+os.Args[2+s])
		CheckError(err)
		/* Aqui não foi definido o endereço do cliente.
		   Usando nil, o próprio sistema escolhe. */
		Conn, err := net.DialUDP("udp", nil, ServerAddr)
		CliConn[s] = Conn
		CheckError(err)
	}
}

func readInput(ch chan string) {
	// Rotina que “escuta” o stdin
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

	ch := make(chan string) // canal que guarda itens lidos do teclado
	go readInput(ch)        // chamar rotina que “escuta” o teclado
	go doServerJob()

	for {
		// Verificar (de forma não bloqueante) se tem algo no stdin (input do terminal)
		select {
		case x, valid := <-ch:
			if valid {
				fmt.Printf("From keyboard: %s \n", x)
				for j := 0; j < nServers; j++ {
					go doClientJob(j, 100)
				}
			} else {
				fmt.Println("Closed channel!")
			}
		default:
			// Fazer nada! Mas não fica bloqueado esperando o teclado
			time.Sleep(time.Second * 1)
		}
		// Esperar um pouco
		time.Sleep(time.Second * 1)
	}
}
