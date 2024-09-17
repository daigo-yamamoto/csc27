package main

import (
    "bufio"
    "encoding/json"
    "fmt"
    "net"
    "os"
    "strconv"
    "sync"
    "time"
)

type Message struct {
    Type     string // "REQUEST" ou "REPLY"
    From     int    // ID do processo remetente
    Clock    int    // Relógio lógico do remetente
    Text     string // Texto da mensagem (opcional)
}

// Variáveis globais
var id int
var clock int
var myPort string
var nServers int
var CliConn []*net.UDPConn
var ServConn *net.UDPConn
var mutex sync.Mutex       // Mutex para proteger o relógio lógico
var replyCount int         // Contador de respostas recebidas
var deferredReplies []int  // Lista de processos para os quais precisamos enviar respostas adiadas
var requestingCS bool      // Indicador se o processo está solicitando a SC
var inCS bool              // Indicador se o processo está na SC

func CheckError(err error) {
    if err != nil {
        fmt.Println("Erro:", err)
        os.Exit(0)
    }
}

func PrintError(err error) {
    if err != nil {
        if err.Error() != "EOF" {
            fmt.Println("Erro:", err)
        }
    }
}

func doServerJob() {
    buf := make([]byte, 1024)
    for {
        n, addr, err := ServConn.ReadFromUDP(buf)
        if err != nil {
            continue
        }

        var msg Message
        err = json.Unmarshal(buf[:n], &msg)
        if err != nil {
            fmt.Println("Erro ao decodificar mensagem:", err)
            continue
        }

        mutex.Lock()
        // Atualiza o relógio lógico
        clock = max(clock, msg.Clock) + 1
        mutex.Unlock()

        switch msg.Type {
        case "REQUEST":
            handleRequest(msg)
        case "REPLY":
            handleReply(msg)
        default:
            fmt.Println("Tipo de mensagem desconhecido:", msg.Type)
        }

        fmt.Printf("Recebido %s de processo %d com clock %d\n", msg.Type, msg.From, msg.Clock)
        _ = addr // Ignorar o endereço neste contexto
    }
}

func handleRequest(msg Message) {
    mutex.Lock()
    defer mutex.Unlock()

    defer sendReply(msg.From)

    // Verifica prioridade
    ourPriority := clock
    senderPriority := msg.Clock

    if inCS || (requestingCS && (senderPriority > ourPriority || (senderPriority == ourPriority && msg.From > id))) {
        // Adiar resposta
        deferredReplies = append(deferredReplies, msg.From)
    } else {
        // Enviar resposta imediatamente
        // A resposta será enviada após o mutex ser liberado
    }
}

func handleReply(msg Message) {
    mutex.Lock()
    defer mutex.Unlock()
    replyCount++
}

func sendReply(dest int) {
    // Envia uma mensagem de REPLY para o processo destino
    msg := Message{
        Type:  "REPLY",
        From:  id,
        Clock: clock,
    }

    jsonMsg, err := json.Marshal(msg)
    if err != nil {
        fmt.Println("Erro ao codificar mensagem:", err)
        return
    }

    idx := getProcessIndex(dest)
    if idx >= 0 && idx < len(CliConn) {
        _, err = CliConn[idx].Write(jsonMsg)
        if err != nil {
            fmt.Println("Erro ao enviar REPLY para processo", dest, ":", err)
        }
    }
}

func requestCS() {
    mutex.Lock()
    if requestingCS || inCS {
        fmt.Println("Já estou aguardando ou dentro da SC. 'x' ignorado.")
        mutex.Unlock()
        return
    }

    clock++
    timestamp := clock
    requestingCS = true
    replyCount = 0
    deferredReplies = []int{}
    mutex.Unlock()

    // Envia REQUEST para todos os outros processos
    msg := Message{
        Type:  "REQUEST",
        From:  id,
        Clock: timestamp,
    }

    jsonMsg, err := json.Marshal(msg)
    if err != nil {
        fmt.Println("Erro ao codificar mensagem:", err)
        return
    }

    for i := 0; i < nServers; i++ {
        _, err = CliConn[i].Write(jsonMsg)
        if err != nil {
            fmt.Println("Erro ao enviar REQUEST para processo", i, ":", err)
        }
    }

    // Aguarda receber REPLY de todos os outros processos
    for {
        mutex.Lock()
        if replyCount == nServers {
            mutex.Unlock()
            break
        }
        mutex.Unlock()
        time.Sleep(100 * time.Millisecond)
    }

    enterCS()
}

func enterCS() {
    mutex.Lock()
    inCS = true
    requestingCS = false
    mutex.Unlock()

    fmt.Println("Entrei na SC")

    // Envia mensagem para o SharedResource
    sendToSharedResource()

    // Simula uso da SC
    time.Sleep(2 * time.Second)

    fmt.Println("Saí da SC")

    mutex.Lock()
    inCS = false
    // Envia respostas adiadas
    for _, proc := range deferredReplies {
        sendReply(proc)
    }
    mutex.Unlock()
}

func sendToSharedResource() {
    // Envia mensagem para o SharedResource
    msg := Message{
        Type:  "CS",
        From:  id,
        Clock: clock,
        Text:  "Oi do processo " + strconv.Itoa(id),
    }

    jsonMsg, err := json.Marshal(msg)
    if err != nil {
        fmt.Println("Erro ao codificar mensagem para SharedResource:", err)
        return
    }

    ServerAddr, err := net.ResolveUDPAddr("udp", "127.0.0.1:10001")
    if err != nil {
        fmt.Println("Erro ao resolver endereço do SharedResource:", err)
        return
    }

    Conn, err := net.DialUDP("udp", nil, ServerAddr)
    if err != nil {
        fmt.Println("Erro ao conectar com SharedResource:", err)
        return
    }
    defer Conn.Close()

    _, err = Conn.Write(jsonMsg)
    if err != nil {
        fmt.Println("Erro ao enviar mensagem para SharedResource:", err)
    }
}

func getProcessIndex(procID int) int {
    idx := -1
    count := 0
    for i := 1; i <= nServers+1; i++ {
        if i == id {
            continue
        }
        if i == procID {
            idx = count
            break
        }
        count++
    }
    return idx
}

func initConnections() {
    var err error
    if len(os.Args) < 4 {
        fmt.Println("Uso: go run Process.go <id> :<porta1> :<porta2> ...")
        os.Exit(1)
    }

    id, err = strconv.Atoi(os.Args[1])
    CheckError(err)
    clock = 0
    requestingCS = false
    inCS = false
    deferredReplies = []int{}

    // Total de processos (incluindo este)
    nTotalServers := len(os.Args) - 2

    // Nossa porta está na posição id + 1 (já que os IDs começam em 1)
    myPort = os.Args[id+1]

    // Número de outros servidores
    nServers = nTotalServers - 1

    // Inicializa conexões com outros servidores
    CliConn = make([]*net.UDPConn, nServers)

    // Configura a conexão do servidor para receber mensagens
    ServerAddr, err := net.ResolveUDPAddr("udp", "127.0.0.1"+myPort)
    CheckError(err)
    ServConn, err = net.ListenUDP("udp", ServerAddr)
    CheckError(err)

    // Configura conexões com outros servidores
    idx := 0
    for i := 1; i <= nTotalServers; i++ {
        if i == id {
            continue // Pula nosso próprio processo
        }
        ServerAddr, err := net.ResolveUDPAddr("udp", "127.0.0.1"+os.Args[i+1])
        CheckError(err)
        Conn, err := net.DialUDP("udp", nil, ServerAddr)
        CheckError(err)
        CliConn[idx] = Conn
        idx++
    }
}

func readInput(ch chan string) {
    // Rotina para escutar o stdin
    reader := bufio.NewReader(os.Stdin)
    for {
        text, _, err := reader.ReadLine()
        if err != nil {
            fmt.Println("Erro ao ler entrada:", err)
            continue
        }
        ch <- string(text)
    }
}

func main() {
    initConnections()
    defer ServConn.Close()
    for _, conn := range CliConn {
        defer conn.Close()
    }

    ch := make(chan string)
    go readInput(ch)
    go doServerJob()

    for {
        select {
        case input, valid := <-ch:
            if valid {
                input = input
                if input == "x" {
                    requestCS()
                } else if procID, err := strconv.Atoi(input); err == nil && procID == id {
                    // Ação interna: incrementa o relógio lógico
                    mutex.Lock()
                    clock++
                    fmt.Println("Ação interna: relógio lógico incrementado para", clock)
                    mutex.Unlock()
                } else {
                    fmt.Println("Entrada inválida:", input)
                }
            } else {
                fmt.Println("Canal fechado!")
            }
        default:
            // Não faz nada, evita bloqueio
            time.Sleep(500 * time.Millisecond)
        }
    }
}

func max(a, b int) int {
    if a > b {
        return a
    }
    return b
}
