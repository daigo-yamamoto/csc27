package mapreduce

import (
	"log"
	"sync"
)

// Schedules map operations on remote workers. This will run until InputFilePathChan
// is closed. If there is no worker available, it'll block.
func (master *Master) schedule(task *Task, proc string, filePathChan chan string) int {
	var (
        wg        sync.WaitGroup
        worker    *RemoteWorker
        operation *Operation
        counter   int
    )

    log.Printf("Scheduling %v operations\n", proc)

    // Collect all file paths from the channel
    filePaths := []string{}
    for filePath := range filePathChan {
        filePaths = append(filePaths, filePath)
    }

    // Initialize total operations
    master.totalOperations = len(filePaths)
    counter = 0

    // Enqueue initial operations
    for _, filePath := range filePaths {
        operation = &Operation{proc, counter, filePath}
        counter++

        worker = <-master.idleWorkerChan
        wg.Add(1)
        go master.runOperation(worker, operation, &wg)
    }

    // Wait for initial operations to complete
    wg.Wait()

    // Process failed operations until all are successful
    for {
        master.operationsMutex.Lock()
        if master.successOperations >= master.totalOperations {
            master.operationsMutex.Unlock()
            break
        }
        master.operationsMutex.Unlock()

        // Get a failed operation to retry
        failedOp := <-master.failedOperationsChan
        worker = <-master.idleWorkerChan
        wg.Add(1)
        go master.runOperation(worker, failedOp, &wg)

        // Wait for the retried operation to complete
        wg.Wait()
    }

    log.Printf("%vx %v operations completed\n", counter, proc)
    return counter
}

// runOperation start a single operation on a RemoteWorker and wait for it to return or fail.
func (master *Master) runOperation(remoteWorker *RemoteWorker, operation *Operation, wg *sync.WaitGroup) {
	defer wg.Done() // Ensure Done is called regardless of success or failure

    var (
        err  error
        args *RunArgs
    )

    log.Printf("Running %v (ID: '%v' File: '%v' Worker: '%v')\n", operation.proc, operation.id, operation.filePath, remoteWorker.id)

    args = &RunArgs{operation.id, operation.filePath}
    err = remoteWorker.callRemoteWorker(operation.proc, args, new(struct{}))

    if err != nil {
        log.Printf("Operation %v '%v' Failed. Error: %v\n", operation.proc, operation.id, err)

        // Send the failed worker to be handled
        master.failedWorkerChan <- remoteWorker

        // Re-enqueue the failed operation
        master.failedOperationsChan <- operation
    } else {
        // Return the worker to the idle pool
        master.idleWorkerChan <- remoteWorker

        // Increment the count of successful operations safely
        master.operationsMutex.Lock()
        master.successOperations++
        master.operationsMutex.Unlock()
    }
}
