package mr

//
// RPC definitions.
//
// remember to capitalize all names.
//

import (
	"os"
	"strconv"
)

// Add your RPC definitions here.
type NullArgs struct{
}

type NullReply struct{
}

type WorkerArgs struct {
	Worker_id   int
	Worker_addr string
}

type WorkerReply struct {
}

type MapArgs struct {
	NReduce    int
	Filename   string
	Maptask_id int
	Worker_id  int
}

type MapReply struct {
	NReduce    int
	Filename   string
	Maptask_id int
}

type ReduceArgs struct {
	Reducetask_id int
	Reducefiles   []string
	Worker_id     int
}

type ReduceReply struct {
	Reducetask_id int
	Reducefiles   []string
}

// Cook up a unique-ish UNIX-domain socket name
// in /var/tmp, for the coordinator.
// Can't use the current directory since
// Athena AFS doesn't support UNIX-domain sockets.
func coordinatorSock() string {
	s := "/var/tmp/5840-mr-"
	s += strconv.Itoa(os.Getuid())
	return s
}

func workerSocket() string {
	s := "/var/tmp/5840-mr-w-"
	s += strconv.Itoa(os.Getpid())
	return s
}
