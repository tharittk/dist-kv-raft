package shardkv

import (
	"sync"
	"sync/atomic"
	"time"

	"6.5840/labgob"
	"6.5840/labrpc"
	"6.5840/raft"
	"6.5840/shardctrler"
)

type Op struct {
	Op        string
	Key       string
	Value     string
	ClientId  int64
	TimeStamp time.Time

	Config    shardctrler.Config
	ShardId   int
	ConfigNum int   // Extra Challenge: Delete Shards
	ShardIds  []int // Extra Challenge: Delete Shards
	ShardData [shardctrler.NShards]map[string]string
	Ack       map[int64]time.Time
}

type ShardKV struct {
	mu      sync.Mutex
	me      int
	rf      *raft.Raft
	applyCh chan raft.ApplyMsg
	dead    int32 // set by Kill()

	make_end     func(string) *labrpc.ClientEnd
	gid          int
	ctrlers      []*labrpc.ClientEnd
	maxraftstate int // snapshot if log grows this big

	commitChan map[int]chan Op

	sm         *shardctrler.Clerk
	config     shardctrler.Config
	prevConfig shardctrler.Config

	data [shardctrler.NShards]map[string]string
	ack  map[int64]time.Time
}

func isOpMatched(from_client Op, from_raft Op) bool {
	// Code hidden
}

func (kv *ShardKV) createSnapshot() []byte {
	// Code hidden
}

func (kv *ShardKV) decodeSnapshot(snapshot []byte) {
	// Code hidden
}

func (kv *ShardKV) checkSnapshot(index int) {
	// Code hidden
}

func (kv *ShardKV) sendToRaftAndWait(from_client Op) (bool, Op) {

	index, _, isLeader := kv.rf.Start(from_client)

	if !isLeader {
		return false, from_client
	}

	kv.mu.Lock()

	opCh, exist := kv.commitChan[index]

	if !exist {
		opCh = make(chan Op, 1)
		kv.commitChan[index] = opCh
	}

	kv.mu.Unlock()

	select {
	// Op arrives after applied to the state machine
	case from_raft := <-opCh:
		return isOpMatched(from_client, from_raft), from_raft
	case <-time.After(500 * time.Millisecond):
		return false, from_client
	}
}

func (kv *ShardKV) isKeyInGroup(key string) bool {
	// Code hidden
}

func (kv *ShardKV) Get(args *GetArgs, reply *GetReply) {
	// Code hidden
}

func (kv *ShardKV) PutAppend(args *PutAppendArgs, reply *PutAppendReply) {
	// Code hidden
}

func (kv *ShardKV) Kill() {
	kv.rf.Kill()
}

func (kv *ShardKV) killed() bool {
	z := atomic.LoadInt32(&kv.dead)
	return z == 1
}

func (kv *ShardKV) runKVServer() {
	for !kv.killed() {

		// Process reply from kv.rf.Start()
		applyMsg := <-kv.applyCh

		op, ok := applyMsg.Command.(Op)

		// conversion will fail if it is a snapshot
		if !ok && applyMsg.SnapshotValid {
			// Code hidden

		} else if applyMsg.CommandValid {

			index := applyMsg.CommandIndex

			kv.mu.Lock()

			switch op.Op {
			case "Get":
				// Code hidden
			case "Put":
				// Code hidden

			case "Append":
				// Code hidden

			case "Reconfigure":
				// Code hidden

			case "DeleteShard":
				// Code hidden

			}
			// Code hidden

		} else {
			// Code hidden
		}
	}
}

func (kv *ShardKV) runConfigPolling() {
	for !kv.killed() {
	// Code hidden
}

type TransferShardArgs struct {
	Num      int
	ShardIds []int
}

type TransferShardReply struct {
	Err       Err
	ShardData [shardctrler.NShards]map[string]string
	Ack       map[int64]time.Time
}

// return the Op in which this server once apply to its state machine,
// it will satisfy the nextConfig specification
func (kv *ShardKV) getReconfigOp(nextConfig shardctrler.Config) (Op, bool, map[int][]int) {
	// Code hidden
}

func (kv *ShardKV) getShardsToTransfer(nextConfig shardctrler.Config) map[int][]int {
	// for each gid, which shard(s) that gid has to send to me

	// Code hidden
}

func (kv *ShardKV) sendTransferShardRequest(gid int, args *TransferShardArgs, reply *TransferShardReply) bool {
	// Try asking for the shard for any server in replica groups
	
	// Code hidden
}

// RPC so call to give the requestor shards
func (kv *ShardKV) TransferShard(args *TransferShardArgs, reply *TransferShardReply) {
	// Code hidden
}

// StartServer() must return quickly, so it should start goroutines
// for any long-running work.
func StartServer(servers []*labrpc.ClientEnd, me int, persister *raft.Persister, maxraftstate int, gid int, ctrlers []*labrpc.ClientEnd, make_end func(string) *labrpc.ClientEnd) *ShardKV {
	// call labgob.Register on structures you want
	// Go's RPC library to marshall/unmarshall.
	labgob.Register(Op{})

	kv := new(ShardKV)
	kv.me = me
	kv.maxraftstate = maxraftstate
	kv.make_end = make_end
	kv.gid = gid
	kv.ctrlers = ctrlers

	kv.applyCh = make(chan raft.ApplyMsg)
	kv.rf = raft.Make(servers, me, persister, kv.applyCh)

	kv.commitChan = make(map[int]chan Op)

	kv.sm = shardctrler.MakeClerk(kv.ctrlers)
	kv.ack = make(map[int64]time.Time)

	for i := 0; i < shardctrler.NShards; i++ {
		kv.data[i] = make(map[string]string)
	}

	kv.decodeSnapshot(kv.rf.ReadSnapshot())

	go kv.runKVServer()
	go kv.runConfigPolling()

	return kv
}

// Extra Challege: Delete Shard

type DeleteShardArgs struct {
	Num      int
	ShardIds []int
}

type DeleteShardReply struct {
	Err Err
}

func (kv *ShardKV) sendDeleteShardRequest(gid int, args *DeleteShardArgs, reply *DeleteShardReply) {
	// Code hidden
}

func (kv *ShardKV) DeleteShard(args *DeleteShardArgs, reply *DeleteShardReply) {
	// Code hidden
}
