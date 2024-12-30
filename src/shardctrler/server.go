package shardctrler

import (
	"log"
	"sync"
	"sync/atomic"
	"time"

	"6.5840/labgob"
	"6.5840/labrpc"
	"6.5840/raft"
)

type ShardCtrler struct {
	mu      sync.Mutex
	me      int
	rf      *raft.Raft
	applyCh chan raft.ApplyMsg
	dead    int32

	configs []Config // indexed by config num
	cache   map[int64]time.Time

	indexChan map[int]chan Op
}

type Op struct {
	// Your data here.
	Command       string
	ArgsJoin      map[int][]string // gid->server
	ArgsLeave     []int            // ([]int)
	ArgsMoveShard int
	ArgsMoveGid   int //(shard, Gid)
	ArgsQuery     int
	ReplyQuery    Config
	ClientId      int64
	TimeStamp     time.Time
}

func (sc *ShardCtrler) sendToRaftAndWait(from_client_op Op) (bool, Op) {
	index, _, isLeader := sc.rf.Start(from_client_op)

	if !isLeader {
		return false, from_client_op
	}

	sc.mu.Lock()

	opCh, exist := sc.indexChan[index]

	if !exist {
		opCh = make(chan Op, 1)
		sc.indexChan[index] = opCh
	}

	sc.mu.Unlock()

	select {
	case from_raft_op := <-opCh:
		return isOpMatched(from_client_op, from_raft_op), from_raft_op
	case <-time.After(500 * time.Millisecond):
		//fmt.Println("Time out")
		return false, from_client_op
	}
}

func isOpMatched(from_client_op Op, from_raft_op Op) bool {
	// Code hidden
}

func (sc *ShardCtrler) Join(args *JoinArgs, reply *JoinReply) {
	// Code hidden
}

func (sc *ShardCtrler) Leave(args *LeaveArgs, reply *LeaveReply) {
	// Code hidden
}

func (sc *ShardCtrler) Move(args *MoveArgs, reply *MoveReply) {
	// Code hidden
}

func (sc *ShardCtrler) Query(args *QueryArgs, reply *QueryReply) {
	// Code hidden
}

// the tester calls Kill() when a ShardCtrler instance won't
// be needed again. you are not required to do anything
// in Kill(), but it might be convenient to (for example)
// turn off debug output from this instance.
func (sc *ShardCtrler) Kill() {
	sc.rf.Kill()
	// Your code here, if desired.
}

func (sc *ShardCtrler) killed() bool {
	z := atomic.LoadInt32(&sc.dead)
	return z == 1
}

// needed by shardkv tester
func (sc *ShardCtrler) Raft() *raft.Raft {
	return sc.rf
}

// servers[] contains the ports of the set of
// servers that will cooperate via Raft to
// form the fault-tolerant shardctrler service.
// me is the index of the current server in servers[].
func StartServer(servers []*labrpc.ClientEnd, me int, persister *raft.Persister) *ShardCtrler {
	sc := new(ShardCtrler)
	sc.me = me

	// first config initialization
	sc.configs = make([]Config, 1)
	sc.configs[0].Groups = map[int][]string{}
	sc.configs[0].Num = 0
	for i := range sc.configs[0].Shards {
		sc.configs[0].Shards[i] = 0
	}

	labgob.Register(Op{})
	sc.applyCh = make(chan raft.ApplyMsg)
	sc.rf = raft.Make(servers, me, persister, sc.applyCh)

	// Your code here.
	sc.indexChan = make(map[int]chan Op)
	sc.cache = make(map[int64]time.Time)

	go sc.runShardCtrler()

	return sc
}

func (sc *ShardCtrler) runShardCtrler() {
	for !sc.killed() {
		applyMsg := <-sc.applyCh

		op, ok := applyMsg.Command.(Op)

		if !ok {
			continue

		} else if applyMsg.CommandValid {
			index := applyMsg.CommandIndex

			sc.mu.Lock()
			lastTime, exist := sc.cache[op.ClientId]
			if !exist || op.TimeStamp.After(lastTime) {
				sc.cache[op.ClientId] = op.TimeStamp

				switch op.Command {
				case "Join":
					sc.handleJoin(&op)
				case "Leave":
					sc.handleLeave(&op)
				case "Move":
					sc.handleMove(&op)
				case "Query":
					sc.handleQuery(&op)
				}
			}

			opCh, exist := sc.indexChan[index]

			if !exist {
				opCh = make(chan Op, 1)
				sc.indexChan[index] = opCh
			}

			opCh <- op

			sc.mu.Unlock()
		} else {
			log.Printf("You should NOT reach here in any circumstances: %v", applyMsg)
		}

	}
}

func (sc *ShardCtrler) handleJoin(op *Op) {
	// Code hidden
}

func (sc *ShardCtrler) handleLeave(op *Op) {
	// Code hidden
}

func (sc *ShardCtrler) handleMove(op *Op) {
	// Code hidden
}

func (sc *ShardCtrler) handleQuery(op *Op) {
	// Code hidden
}
