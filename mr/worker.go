package mr

import (
	"encoding/json"
	"fmt"
	"hash/fnv"
	"io"
	"log"
	"net"
	"net/http"
	"net/rpc"
	"os"
	"sort"
	"strconv"
	"time"
)

type WorkerServer struct {
	mapf    func(string, string) []KeyValue
	reducef func(string, []string) string
}

// Map functions return a slice of KeyValue.
type KeyValue struct {
	Key   string
	Value string
}

// for sorting by key.
type ByKey []KeyValue

// for sorting by key.
func (a ByKey) Len() int           { return len(a) }
func (a ByKey) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a ByKey) Less(i, j int) bool { return a[i].Key < a[j].Key }

// use ihash(key) % NReduce to choose the reduce
// task number for each KeyValue emitted by Map.
func ihash(key string) int {
	h := fnv.New32a()
	h.Write([]byte(key))
	return int(h.Sum32() & 0x7fffffff)
}

// main/mrworker.go calls this function.
func Worker(mapf func(string, string) []KeyValue,
	reducef func(string, []string) string) {

	worker_server := WorkerServer{mapf: mapf, reducef: reducef}

	worker_server.startServer()

	addr := workerSocket()

	register_info := WorkerArgs{Worker_id: os.Getpid(), Worker_addr: addr}

	_na := NullArgs{}
	_nr := NullReply{}
	
	call("Coordinator.RegisterWorker", register_info, &_nr)

	ma := MapArgs{Worker_id: os.Getpid()}

	for {
		mr := MapReply{}

		get_task := call("Coordinator.GiveMapTask", ma, &mr)

		if get_task {
			map_exec(mapf, mr)
		} else {
			break
		}
	}


	ra := ReduceArgs{Worker_id: os.Getpid()}

	for {
		rr := ReduceReply{}

		get_task := call("Coordinator.GiveReduceTask", ra, &rr)

		if get_task {
			if rr.Reducetask_id == -1 {
				//fmt.Println("Wait for Map all done ...sleep")
				time.Sleep(3 * time.Second)
			} else {
				//fmt.Println ("Reduce Exec")
				reduce_exec(reducef, rr)
			}
		} else {

		}
	}

	exit := false
	for !exit {
		ok := call("Coordinator.isExit", _na, &_nr)
		if !ok { //coordinator already gone
			exit = true
		}

	}

}

func map_exec(mapf func(string, string) []KeyValue, mapReply MapReply) {
	kva := perform_map(mapf, mapReply.Filename)
	bucket_by_intermediate_key(kva, mapReply.Maptask_id, mapReply.NReduce)

	mapArgs := MapArgs{Maptask_id: mapReply.Maptask_id}
	call("Coordinator.CheckLastMapDone", mapArgs, &mapReply)

}

func perform_map(mapf func(string, string) []KeyValue, filename string) []KeyValue {
	file, err := os.Open(filename)
	if err != nil {
		log.Fatalf("Worker cannot open %v", filename)
	}

	content, err := io.ReadAll(file)

	if err != nil {
		log.Fatalf("Worker cannot read %v", filename)
	}

	file.Close()

	kva := mapf(filename, string(content))

	return kva
}

func bucket_by_intermediate_key(kva []KeyValue, task_id int, nReduce int) {

	fileHandles := make(map[int]*os.File)
	encoderHandles := make(map[int]*json.Encoder)

	for i := 0; i < len(kva); i++ {
		ibucket := ihash(kva[i].Key) % nReduce

		// Check if the file for this bucket is already opened, if not open it
		if _, ok := fileHandles[ibucket]; !ok {
			// Open the file in append mode, create it if it doesn't exist
			ofile, err := os.OpenFile("mr-"+strconv.Itoa(task_id)+"-"+
				strconv.Itoa(ibucket), os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
			if err != nil {
				fmt.Println("Error opening file:", err)
				continue
			}

			fileHandles[ibucket] = ofile
			encoderHandles[ibucket] = json.NewEncoder(ofile)
		}

		enc := encoderHandles[ibucket]
		enc.Encode(&kva[i])
	}

	for _, ofile := range fileHandles {
		ofile.Close()
	}

}

func reduce_exec(reducef func(string, []string) string, reduceReply ReduceReply) {
	ofile_name := "mr-out-" + strconv.Itoa(reduceReply.Reducetask_id)
	kva := read_intermediates(reduceReply.Reducefiles)
	perform_reduce(reducef, kva, ofile_name)

	reduceArgs := ReduceArgs{Reducetask_id: reduceReply.Reducetask_id}
	call("Coordinator.CheckLastReduceDone", reduceArgs, &reduceReply)

}

// read all and sort
func read_intermediates(filenames []string) []KeyValue {
	kva := []KeyValue{}

	for _, filename := range filenames {
		ofile, _ := os.Open(filename)
		dec := json.NewDecoder(ofile)

		for {
			var kv KeyValue
			if err := dec.Decode(&kv); err != nil {
				break
			}
			kva = append(kva, kv)
		}
	}
	sort.Sort(ByKey(kva))

	//fmt.Println ("Success: Reducing read from intermeidates:: ", filenames, ":::", kva)

	return kva
}

func perform_reduce(reducef func(string, []string) string,
	intermediate []KeyValue, ofile_name string) {
	
	ofile, _ := os.OpenFile(ofile_name, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)

	//fmt.Println("Perfrom Reduce, Opening ofile_name: ", ofile_name)

	i := 0
	for i < len(intermediate) {
		j := i + 1
		for j < len(intermediate) && intermediate[j].Key == intermediate[i].Key {
			j++
		}
		values := []string{}
		for k := i; k < j; k++ {
			values = append(values, intermediate[k].Value)
		}
		output := reducef(intermediate[i].Key, values)

		// this is the correct format for each line of Reduce output.
		fmt.Fprintf(ofile, "%v %v\n", intermediate[i].Key, output)

		i = j
	}
	//fmt.Println("Done Reduce, Opening ofile_name: ", ofile_name)

}

// send an RPC request to the coordinator, wait for the response.
// usually returns true.
// returns false if something goes wrong.
func call(rpcname string, args interface{}, reply interface{}) bool {
	// c, err := rpc.DialHTTP("tcp", "127.0.0.1"+":1234")
	sockname := coordinatorSock()
	c, err := rpc.DialHTTP("unix", sockname)
	if err != nil {
		log.Fatal("dialing:", err)
	}
	defer c.Close()

	err = c.Call(rpcname, args, reply)
	if err == nil {
		return true
	}

	//fmt.Println(err)

	return false
}

// Coordinator call RPC to execute the task in place of died worker
func (w *WorkerServer) DoMapTask(args *MapArgs, nr *NullReply) error {

	// wrap arg to MapReply
	reply := MapReply{NReduce: args.NReduce,
		Filename:   args.Filename,
		Maptask_id: args.Maptask_id}

	fmt.Println("Redo Maptask_id", reply.Maptask_id, "by: ", os.Getpid())

	map_exec(w.mapf, reply)

	return nil
}

func (w *WorkerServer) DoReduceTask(args *ReduceArgs, nr *NullReply) error {

	// wrap arg to ReduceReply
	reply := ReduceReply{Reducetask_id: args.Reducetask_id, Reducefiles: args.Reducefiles}

	fmt.Println("Redo Reduce_id", reply.Reducetask_id)

	reduce_exec(w.reducef, reply)

	return nil
}

func (w *WorkerServer) startServer() {
	rpc.Register(w)
	rpc.HandleHTTP()

	sockname := workerSocket()
	os.Remove(sockname) // Ensure no leftover socket file

	l, err := net.Listen("unix", sockname)
	if err != nil {
		log.Fatal("Worker listen error:", err)
	}
	go http.Serve(l, nil) // Start the RPC server in a goroutine
}

func (w *WorkerServer) SayHi(args *NullArgs, reply *NullReply) error {
	fmt.Println("Worker: ", os.Getpid(), "Got message !")
	return nil
}
