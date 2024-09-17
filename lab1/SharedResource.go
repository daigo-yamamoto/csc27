package main

import (
    "encoding/json"
    "fmt"
    "net"
    "os"
)

type Message struct {
    Type  string
    From  int
    Clock int
    Text  string
}

func CheckError(err error) {
    if err != nil {
        fmt.Println("Erro:", err)
        os.Exit(0)
    }
}

func main() {
    Address, err := net.ResolveUDPAddr("udp", ":10001")
    CheckError(err)
    Connection, err := net.ListenUDP("udp", Address)
    CheckError(err)
    defer Connection.Close()

    buf := make([]byte, 1024)
    for {
        n, addr, err := Connection.ReadFromUDP(buf)
        if err != nil {
            continue
        }

        var msg Message
        err = json.Unmarshal(buf[:n], &msg)
        if err != nil {
            fmt.Println("Erro ao decodificar mensagem:", err)
            continue
        }

        fmt.Printf("Recurso Compartilhado recebeu mensagem do processo %d com clock %d: %s\n", msg.From, msg.Clock, msg.Text)
        _ = addr // Ignorar o endereço neste contexto
    }
}
