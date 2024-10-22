---
title: "Modelo de programação MapReduce"
author: 
  - "Bruno Daigo Yamamoto"
  - "Francisco Gabriel"
date: "Outubro 2024"
institution: "Instituto Tecnológico de Aeronáutica"
geometry: "left=1.5in, right=1.5in, top=1in, bottom=1in"
header-includes:
  - \usepackage{fancyhdr}
  - \pagestyle{fancy}
  - \fancyhead[L]{Modelo de programação MapReduce}
  - \fancyhead[R]{Outubro 2024}
  - \fancyfoot[C]{\thepage}
  - \usepackage{indentfirst}
  - \setlength{\parindent}{1.5em}
---

# Parte 1: Implementação Sequencial

## Função mapFunc

```go
func mapFunc(input []byte) (result []mapreduce.KeyValue) {
    text := string(input)

    words := strings.FieldsFunc(text, func(c rune) bool {
        return !unicode.IsLetter(c) && !unicode.IsNumber(c)
    })

    result = make([]mapreduce.KeyValue, 0)

    for _, word := range words {
        word = strings.ToLower(word)
        kv := mapreduce.KeyValue{Key: word, Value: "1"}
        result = append(result, kv)
    }

    return result
}
```

- Conversão do Chunk: Transforma os bytes em string para processamento.
- Divisão em Palavras: Utiliza strings.FieldsFunc com um delimitador personalizado.
- Normalização: Converte as palavras para minúsculas.
- Emissão dos Pares: Cada palavra é associada ao valor "1".

### Teste da função mapFunc

```bash
    $ ./wordcount -mode sequential -file files/teste.txt -chunksize 100 -reducejobs 2
```

![Teste da função mapFunc](images/mapFunc.png)

O arquivo de texto foi dividido em pedaços de 100 bytes, conforme especificado pelo parâmetro chunksize. Cada um desses pedaços foi processado de forma independente. Para cada pedaço, a função mapFunc é chamada, onde o conteúdo do pedaço é lido e cada palavra é identificada, convertida para minúsculas, e associada ao valor 1, indicando que a palavra apareceu uma vez.

## Função reduceFunc

```go
    func reduceFunc(input []mapreduce.KeyValue) (result []mapreduce.KeyValue) {
    mapAux := make(map[string]int)

    for _, item := range input {
        value, err := strconv.Atoi(item.Value)
        if err != nil {
            continue
        }
        mapAux[item.Key] += value
    }

    result = make([]mapreduce.KeyValue, 0, len(mapAux))
    for key, count := range mapAux {
        kv := mapreduce.KeyValue{Key: key, Value: strconv.Itoa(count)}
        result = append(result, kv)
    }

    return result
    }
```

- Agrupamento: Utiliza um mapa auxiliar para acumular as contagens.
- Conversão de Valores: Converte os valores de string para inteiro.
- Construção do Resultado: Cria uma lista de pares chave-valor com as contagens finais.

### Teste da funcao reduceFunc

```bash
    $ ./wordcount -mode sequential -file files/teste.txt -chunksize 100 -reducejobs 2
```

![Teste da função mapFunc](images/reduceFunc.png)

A função `reduceFunc` desempenha o papel de consolidar os resultados intermediários gerados pela fase de mapeamento no MapReduce. No caso do problema de contagem de palavras, o `reduceFunc` recebe como entrada uma palavra (chave) e uma lista de valores que representam o número de vezes que essa palavra foi encontrada em diferentes partes do arquivo durante a fase Map. O funcionamento da função consiste em percorrer essa lista de valores e somar todas as ocorrências da palavra associada, gerando um par chave-valor final onde a chave é a palavra e o valor é o total de ocorrências. Por exemplo, a palavra "teste", que aparece três vezes no total (duas vezes no primeiro chunk e uma vez no segundo), será somada para resultar em {teste, 3}, como pode ser visto na figura acima. Ao final da fase de redução, todas as palavras são agregadas e cada uma tem sua contagem totalizada de acordo com os dados recebidos da fase Map. Esse processo garante que todas as palavras encontradas no arquivo tenham suas contagens combinadas e corretamente consolidadas para a saída final.

## Variação de `chunksize` e `reducejobs`

### Experimento 1: Variando chunksize

Configuração A: 
```bash
    ./wordcount -mode sequential -file files/pg1342.txt -chunksize 1024 -reducejobs 2
```
- Observações:
  - O arquivo foi dividido em chunks de 1 KB.
  - Número maior de tarefas de map.

Configuração B:
```bash
    ./wordcount -mode sequential -file files/pg1342.txt -chunksize 4096 -reducejobs 2
```
- Observações:
  - Chunks de 4 KB.
  - Menos tarefas de map.

Análise:

- Desempenho:
  - Com chunks menores, há mais paralelismo potencial, mas também maior sobrecarga na criação de tarefas.
  - Com chunks maiores, a sobrecarga é reduzida, mas cada tarefa processa mais dados.

### Experimento 2: Variando reducejobs

Configuração A: 
```bash
    ./wordcount -mode sequential -file files/pg1342.txt -chunksize 2048 -reducejobs 2
```

Configuração B:
```bash
    ./wordcount -mode sequential -file files/pg1342.txt -chunksize 2048 -reducejobs 4
```
Análise:

- Distribuição das Palavras:
  - Com mais reduce jobs, as palavras são distribuídas entre mais tarefas.
- Impacto no Desempenho:
  - Em modo sequencial, o benefício é limitado.
  - Em um ambiente paralelo, mais reduce jobs podem melhorar o desempenho.


# Parte 2

Nessa parte veremos a execução do código MapReduce no modo distribuído, utilizando múltiplos workers e introduzindo falhas simuladas para testar o comportamento do sistema. O objetivo é verificar se o código responde adequadamente a falhas e se as operações Map e Reduce são corretamente distribuídas entre os workers.

## Execução dos testes

### Simular falha em operação de Map com um worker

Neste teste, vamos executar o programa com um worker falho que simula uma falha ao processar a terceira operação de map.

Comandos utilizados:

1. Inicie o worker falho:

```bash
./wordcount -mode distributed -type worker -port 50001 -fail 3
```

2. Inicie um segundo worker normal:

```bash
./wordcount -mode distributed -type worker -port 50002
```

3. Inicie o master:

```bash
./wordcount -mode distributed -type master -file files/teste.txt -chunksize 102400 -reducejobs 5
```
![Simulação da falha em operação de Map com um worker](images/falho_map.png)

Em que no terminal da esquerda foi inserido o primeiro comando, no do meio o segundo e no da direita o terceiro comando.

Pode-se observar que:

- O worker 1 falha após a terceira tarefa de map.
- O master detecta a falha e realoca as operações restantes para o worker 2.

### Simular falha em operação de Reduce

Neste teste a falha será induzida durante uma operação de reduce.

Comandos utilizados:

1. Inicie o worker falho:

```bash
./wordcount -mode distributed -type worker -port 50001 -fail 2
```

2. Inicie um segundo worker normal:

```bash
./wordcount -mode distributed -type worker -port 50002
```

3. Inicie o master:

```bash
./wordcount -mode distributed -type master -file files/teste.txt -chunksize 102400 -reducejobs 5
```
![Simulação da falha em operação de Reduce](images/falho_reduce.png)

Em que no terminal da esquerda foi inserido o primeiro comando, no do meio o segundo e no da direita o terceiro comando.

Pode-se observar que:

- O worker falha durante uma operação reduce.
- O master detecta a falha e aloca a tarefa reduce para o outro worker.

### Operações com múltiplos workers falhos e parâmetros variáveis

Neste teste, usaremos diferentes valores de `chunksize` e `reducejobs`, além de incluir dois workers falhos.

Comandos:

Inicie dois workers falhos:

```bash
./wordcount -mode distributed -type worker -port 50001 -fail 3
./wordcount -mode distributed -type worker -port 50002 -fail 4
```

Inicie um terceiro worker normal:

```bash
./wordcount -mode distributed -type worker -port 50003
```

Inicie o master com:

```bash
./wordcount -mode distributed -type master -file files/pg1342.txt -chunksize 204800 -reducejobs 3
```

![Simulação da falha em múltiplos workers](images/falho_multi.png)

Pode-se observar que:

- O master redistribui as tarefas falhadas dos dois workers.
- As tarefas são concluídas corretamente, mesmo com múltiplas falhas.

# Conclusão

Ao final dos testes, o sistema se mostrou robusto ao lidar com falhas em workers, redistribuindo corretamente as operações de Map e Reduce.

