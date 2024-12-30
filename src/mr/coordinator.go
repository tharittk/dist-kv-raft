package mr

import (
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/rpc"
	"os"
	"strconv"
	"sync"
	"time"
)

type Coordinator struct {
	// Your definitions here.
	filenames      []string
	counter_map    int
	counter_reduce int
	nReduce        int
	mu             sync.Mutex
	done_map       int
	done_reduce    int

	//worker
	worker_book     map[int]string
	map_complete    []bool
	reduce_complete []bool
}

// start a thread that listens for RPCs from worker.go
func (c *Coordinator) server() {
	rpc.Register(c)
	rpc.HandleHTTP()

	sockname := coordinatorSock()
	os.Remove(sockname)
	l, e := net.Listen("unix", sockname)
	if e != nil {
		log.Fatal("listen error:", e)
	}
	go http.Serve(l, nil)
}

func (c *Coordinator) Done() bool {
	//fmt.Println("Coordinator done map ", c.done_map, "/", len(c.filenames))
	//fmt.Println("Coordinator done reduce ", c.done_reduce, "/", c.nReduce)

	return c.done_map == len(c.filenames) && c.done_reduce == c.nReduce
}

func MakeCoordinator(files []string, nReduce int) *Coordinator {
	c := Coordinator{filenames: files, counter_map: 0,
		counter_reduce: 0, nReduce: nReduce,
		done_map: 0, done_reduce: 0, worker_book: make(map[int]string),
		map_complete: make([]bool, len(files)), reduce_complete: make([]bool, nReduce)}

	for i := 0; i < len(files); i++ {
		c.map_complete[i] = false
	}
	for i := 0; i < nReduce; i++ {
		c.reduce_complete[i] = false
	}

	c.server()

	return &c
}

func (c *Coordinator) RegisterWorker(args *WorkerArgs, reply *NullReply) error {
	c.mu.Lock()
	c.worker_book[args.Worker_id] = args.Worker_addr
	c.mu.Unlock()

	return nil
}

func (c *Coordinator) GiveMapTask(args *MapArgs, reply *MapReply) error {

	if c.counter_map >= len(c.filenames) {
		return io.EOF
	} else {
		c.mu.Lock()
		reply.NReduce = c.nReduce
		reply.Filename = c.filenames[c.counter_map]
		reply.Maptask_id = c.counter_map
		c.counter_map++
		go c.spawn_map_monitor(args.Worker_id, reply.Maptask_id)
		c.mu.Unlock()

		return nil
	}
}

func (c *Coordinator) spawn_map_monitor(worker_id int, task_id int) {
	time.Sleep(10 * time.Second)

	for {
		if c.map_complete[task_id] {
			break
		} else {

			fmt.Println(" Not done @ Map task ", task_id, " worker id: ", worker_id)

			c.callWorkerRedoMap(task_id)
			time.Sleep(10 * time.Second)
		}
	}
}

func (c *Coordinator) GiveReduceTask(args *ReduceArgs, reply *ReduceReply) error {
	if !(c.done_map == len(c.filenames)) {
		reply.Reducetask_id = -1
		return nil
	}

	if c.counter_reduce >= c.nReduce {
		return io.EOF
	} else {
		c.mu.Lock()
		reduce_files := make([]string, c.counter_map)
		for i := 0; i < c.counter_map; i++ {
			filename := "mr-" + strconv.Itoa(i) + "-" + strconv.Itoa(c.counter_reduce)
			reduce_files[i] = filename
		}
		reply.Reducefiles = reduce_files
		reply.Reducetask_id = c.counter_reduce

		c.counter_reduce++

		go c.spawn_reduce_monitor(args.Worker_id, reply.Reducetask_id, reduce_files)

		c.mu.Unlock()

		return nil
	}
}

func (c *Coordinator) spawn_reduce_monitor(worker_id int, task_id int, rfiles []string) {
	time.Sleep(10 * time.Second)

	for {
		if c.reduce_complete[task_id] {
			break
		} else {

			fmt.Println(" Not done @ Reduce task ", task_id, " worker id: ", worker_id)

			c.callWorkerRedoReduce(task_id, rfiles)
			time.Sleep(10 * time.Second)
		}
	}
}

var ErrMapNotCompleted = errors.New("map tasks are not completed")
var ErrorNotAllReduceComplete = errors.New("not all Reduce complete")

func (c *Coordinator) CheckLastMapDone(args *MapArgs, reply *MapReply) error {

	if !c.map_complete[args.Maptask_id] {

		c.map_complete[args.Maptask_id] = true
		c.mu.Lock()
		c.done_map += 1
		c.mu.Unlock()
	}

	if c.done_map == len(c.filenames) {
		fmt.Println("Map done @Total:: ", c.done_map)
		return nil
	}
	return ErrMapNotCompleted
}

func (c *Coordinator) CheckLastReduceDone(args *ReduceArgs, reply *ReduceReply) error {

	if !c.reduce_complete[args.Reducetask_id] {
		c.reduce_complete[args.Reducetask_id] = true
		c.mu.Lock()
		c.done_reduce += 1
		c.mu.Unlock()
	}

	if c.done_reduce == c.nReduce {
		fmt.Println("Reduce done @Total: ", c.done_reduce)
		return nil
	} else {
		return ErrorNotAllReduceComplete
	}

}

var ErrNotEveryoneFinish = errors.New("not everyone finishes")

func (c *Coordinator) isExit(args *NullArgs, reply *NullReply) error {
	if c.Done() {
		return nil
	} else {
		return ErrNotEveryoneFinish
	}
}

func (c *Coordinator) callWorkerRedoMap(map_id int) {

	args := MapArgs{NReduce: c.nReduce,
		Filename:   c.filenames[map_id],
		Maptask_id: map_id}

	nr := NullReply{}

	c.callWorker("WorkerServer.DoMapTask", args, &nr)
}

func (c *Coordinator) callWorkerRedoReduce(reduce_id int, reduce_files []string) {

	args := ReduceArgs{Reducetask_id: reduce_id, Reducefiles: reduce_files}
	nr := NullReply{}

	c.callWorker("WorkerServer.DoReduceTask", args, &nr)
}

func (c *Coordinator) callWorker(rpcname string, args interface{}, reply interface{}) bool {

	c.mu.Lock()

	var addr string
	toDelete := []int{}
	for pid, workerAddr := range c.worker_book {
		conn, err := rpc.DialHTTP("unix", workerAddr)
		if err != nil {
			fmt.Println("Dead connection::", pid, err)
			toDelete = append(toDelete, pid) // Mark for deletion
		} else {
			conn.Close()
			addr = workerAddr // Pick the first available worker
			break
		}
	}

	// Safely delete unavailable workers
	for _, pid := range toDelete {
		delete(c.worker_book, pid)
	}

	if addr == "" {
		c.mu.Unlock()
		log.Fatal("No available workers in worker_book")
	}

	fmt.Println("Remaining workers:", len(c.worker_book))
	c.mu.Unlock()

	// RPC Call to the selected worker
	conn, err := rpc.DialHTTP("unix", addr)
	if err != nil {
		log.Fatal("Dead connection during RPC::", err)
	}
	defer conn.Close()

	err = conn.Call(rpcname, args, reply)
	if err == nil {
		return true
	}

	fmt.Println("RPC error:", err)
	return false
}
