package shardkv

import (
	"time"

	"6.5840/shardctrler"
)

//
// Sharded key/value server.
// Lots of replica groups, each running Raft.
// Shardctrler decides which group serves each shard.
// Shardctrler may change shard assignment from time to time.
//
// You will have to modify these definitions.
//

const (
	OK              = "OK"
	ErrNoKey        = "ErrNoKey"
	ErrWrongGroup   = "ErrWrongGroup"
	ErrWrongLeader  = "ErrWrongLeader"
	ErrWrongVersion = "ErrWrongVersion"
	ErrNotReady = "ErrNotReady"
)

type Err string

// Put or Append
type PutAppendArgs struct {
	// You'll have to add definitions here.
	Key       string
	Value     string
	Op        string // put or append
	ClientId  int64
	TimeStamp time.Time
}

type PutAppendReply struct {
	Config shardctrler.Config
	Err    Err
}

type GetArgs struct {
	Key       string
	ClientId  int64
	TimeStamp time.Time
}

type GetReply struct {
	Err   Err
	Value string
}
