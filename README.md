# Building a Fault-Tolerant, Sharded Key-Value Store using Raft in Go 

This repository documents the design and implementation of a Raft-based **fault-tolerant, sharded key-value store**. This is the semester-long project for MIT's 6.824 Distributed Systems course - it is [free to the public](https://pdos.csail.mit.edu/6.824/schedule.html).

The system architecture is built from the ground up:
1. Raft Consensus: A consensus protocol to manage replicated logs and leader election.
2. Key-Value Server: A linearizable state machine built on top of Raft.
3. Shard Controller: A fault-tolerant configuration manager for data distribution.

```markdown
Note: To respect the academic integrity (I'm not an MIT student) of the course, this write-up focuses on
architectural design and high-level logic rather than providing the full, line-by-line source code.
```

### Project Progression
The implementation follows the sequence of the MIT 6.824 labs:

- [x] Lab 1: MapReduce (Batch processing implementation)

- [x] Lab 2: KV Server (Basic single-server setup)

- [x] Lab 3: Raft Consensus (Leader Election, Log Replication, Persistence, Compaction)

- [x] Lab 4: Fault-Tolerant KV (Linearizable operations with snapshots)

- [x] Lab 5: Sharded KV (Shard Controller, Data Movement, Garbage Collection)

👉 Just so you know, I try my best not to tell the actual working code line-by-line. It is beneficial for people who take the class, for leisure or grade, to go through debugging themselves instead of following a thought-through pseudocode.

## The Core: Raft Consensus Algorithm 

At the core of any fault-tolerant system is a consensus algorithm. I implemented the [Raft consensus algorithm](https://raft.github.io/), which provides a way for a group of servers to agree on a single, ordered log of operations, even in the face of failures.

### Data Structures & State

The state of each Raft peer is captured in the `Raft` struct. It holds all the persistent and volatile state described in the Raft paper, along with channels for coordinating the server's state machine. The channel is a preferred way for Go for inter-process communication (ipc). In other languages, such as Python, `queue.Queue()` along with `event` can be used in place of Go's channels as well.

Actually, I discovered later that [gRPC](https://grpc.io/) does all this heavy lifting. But I think building Raft from Go's channel gives the best flavor of it.

```go
type Raft struct {
	mu        sync.Mutex          // Protects shared access to state
	peers     []*labrpc.ClientEnd // RPC endpoints of all peers
	persister *Persister          // Holds persisted state (WAL)
	me        int                 // Index into peers[]

	// Persistent state
	currentTerm int
	votedFor    int
	logs        []LogEntry

	// Volatile state
	commitIndex int
	lastApplied int

	// Leader-specific volatile state
	nextIndex  []int
	matchIndex []int

	// Event Channels
	applyCh     chan ApplyMsg
	winElectCh  chan bool
	heartbeatCh chan bool
}

type LogEntry struct {
	Term    int
	Command interface{}
}
```

### The Raft State Machine

The server lifecycle is controlled by runServer, a main loop that switches behavior based on the current state (Follower, Candidate, Leader). This cleanly isolates the logic for election timeouts and heartbeats.

Here is the illustration [credit](https://blog.kezhuw.name/2018/03/20/A-step-by-step-approach-to-raft-consensus-algorithm):

<p align="center">
<img width="451" height="232" alt="image" src="https://github.com/user-attachments/assets/a1fd8168-704e-4f94-bab2-b406215bf300" />
</p>

```go
// main server loop.
func (rf *Raft) runServer() {
	for !rf.killed() {
		rf.mu.Lock()
		state := rf.state
		rf.mu.Unlock()

		switch state {
		case Leader:
			select {
			// receive higher-numbered term
			case <-rf.stepDownCh:
			// heartbeat
			case <-time.After(30 * time.Millisecond):
				rf.mu.Lock()
				rf.broadcastAppendEntries()
				rf.mu.Unlock()
			}
		case Follower:
			select {
			case <-rf.grantVoteCh:
			case <-rf.heartbeatCh:
			// election timed-out
			case <-time.After(rf.getElectionTimeout() * time.Millisecond):
				rf.convertToCandidate(Follower)
			}
		case Candidate:
			select {
			case <-rf.stepDownCh:
			case <-rf.winElectCh:
				rf.convertToLeader()
			case <-time.After(rf.getElectionTimeout() * time.Millisecond):
				rf.convertToCandidate(Candidate)
			}
		}
	}
}
```

-   **Followers** wait for heartbeats (`AppendEntries` RPCs) from a leader. If a randomized election timeout elapses without receiving one, the follower transitions to a `Candidate`.
-   **Candidates** increment the term, vote for themselves, and request votes from peers. They can either win the election and become a `Leader`, lose the election and step down to a `Follower` if they see a higher term, or time out and start a new election.
-   **Leaders** send periodic heartbeats to all followers to maintain authority and replicate log entries.

### Leader Election (3A)

Elections are driven by randomized timeouts. This simple randomization policy effectively prevents "split votes" and simplifies system reasoning (**I appreciate this more after I studied Paxos months after at my university 🔥**).

The RequestVote RPC ensures safety: a peer grants a vote only if the candidate's log is at least as up-to-date as its own.

The `RequestVote` RPC is the core of this process. A candidate calls this on other peers, and a peer will grant its vote only if it hasn't already voted in the current term and if the candidate's log is at least as up-to-date as its own.

```go
func (rf *Raft) RequestVote(args *RequestVoteArgs, reply *RequestVoteReply) {
	rf.mu.Lock()
	defer rf.mu.Unlock()

    // 1. Reject if candidate has stale term
	if args.Term < rf.currentTerm {
		reply.VoteGranted = false
		return
	}

    // 2. Grant vote if we haven't voted yet AND candidate's log is up-to-date
	if (rf.votedFor < 0 || rf.votedFor == args.CandidateId) &&
		rf.isLogUpToDate(args.LastLogIndex, args.LastLogTerm) {
		reply.VoteGranted = true
		rf.votedFor = args.CandidateId
		rf.sendToChannel(rf.grantVoteCh, true)
	}
}
```

### Log Replication (3B)

Once a leader is elected, it's responsible for replicating its log to all followers. This is done via the `AppendEntries` RPC. The leader sends new log entries to followers, but critically, it also uses this RPC as a heartbeat to prevent followers from starting new elections by sending empty log entry.

The core of log consistency is the check on `PrevLogIndex` and `PrevLogTerm` (RPC arg). A follower only accepts new entries if they are contiguous with its existing log. If not, it rejects the request, and the leader decrements its `nextIndex` for that follower and tries again.

This process maintains the Raft's invariant that **in the system, the log at the same index must have a matching term**. The leader decrements its `nextIndex` means that for that particular follower, the leader
must send earlier log(s) - which potentially replace conflicting logs in the follower.

When a follower’s log is inconsistent with the leader’s, Raft needs to find the first index where their logs agree, then overwrite from there onward.
Without optimization, the leader would decrement nextIndex by 1 each time — slow if there are many entries that mismatch.

```markdown
Hence, there is an optimization to have the follower include its `ConflictIndex` and `ConflictTerm` in the reply
so that the leader can decrement `nextIndex` and re-send once for one conflicting term.
```

```go
func (rf *Raft) AppendEntries(args *AppendEntriesArgs, reply *AppendEntriesReply) {
    // ... term checks ...

  // Consistency Check
	if args.PrevLogIndex > rf.getLastIndex() || rf.logs[args.PrevLogIndex].Term != args.PrevLogTerm {
    // Optimization: Return conflict index to allow Leader to back up quickly
		reply.Success = false
		return
	}

  // Truncate conflict and Append new logs
	rf.logs = append(rf.logs, args.Entries...)
	reply.Success = true

    // Commit
	if args.LeaderCommit > rf.commitIndex {
		rf.commitIndex = min(args.LeaderCommit, rf.getLastIndex())
		go rf.applyLogs() // Apply to state machine
	}
}
```
From the code, we can see that there are 2 cases:

**Case 1 — Leader has entries with that term T**

Follower’s conflicting term T might be shorter or longer than the leader’s copy. But if both have entries in term T, they might still agree up to the last occurrence of T in the leader’s log.
There’s **no need to delete all entries from that term — only what follows it**. So by jumping to the last index of term T in the leader’s log, we ensure that all entries before that are definitely correct.


**Case 2 — Leader does NOT have entries with that term T**

Then the follower’s entire conflicting term T is “garbage” (no such term in the leader’s history).
The leader should jump to the follower’s ConflictIndex — i.e., delete that whole term range.


### Persistence (3C, 3D)
To survive crashes, a Raft server must persist its `currentTerm`, `votedFor`, and `logs` to stable storage before responding to RPCs.

```go
// save Raft's persistent state to stable storage,
func (rf *Raft) persist() {
	w := new(bytes.Buffer)
	e := labgob.NewEncoder(w)
	e.Encode(rf.currentTerm)
	e.Encode(rf.votedFor)
	e.Encode(rf.logs)
	data := w.Bytes()
	rf.persister.Save(data, rf.persister.snapshot)
}
```

Upon server restart, the server reads the persistent state right away. But this alone is not enough since the `commitIndex` is not part of the persistent state by design. To "let the server get up to speed", the leader, upon the successful election, appends the `no-op` log entry and broadcasts it. Raft takes advantage of its side-effect that the restarted server will replay its commit (from start to the leader's `commitIndex`).

### Snapshotting (4A, 4B) 📸

In a long-running system, the Raft log cannot grow indefinitely. We use Snapshotting to trim old log entries once the application state machine has safely processed and persisted them.

The service layer (KV Store) determines when to snapshot. When triggered, Raft discards log entries up to a specific index, records the `snapLastIndex` and `snapLastTerm` (to maintain log consistency for future `AppendEntries`), and persists both the compacted log and the application snapshot.

```go
// Service notifies Raft that log[0..index] is now snapshotted.
func (rf *Raft) Snapshot(index int, snapshot []byte) {
    if index <= rf.snapLastIndex { return }

    rf.mu.Lock()
    defer rf.mu.Unlock()

    // Save the term of the last entry included in the snapshot
    rf.snapLastTerm = rf.logs[index-rf.snapLastIndex].Term
    
    // Truncate the log: keep only entries following the snapshot
    rf.logs = append([]LogEntry{{Term: rf.snapLastTerm}},
                     rf.logs[index-rf.snapLastIndex+1:]...)

    rf.snapLastIndex = index
    rf.persister.Save(rf.persister.ReadRaftState(), snapshot)
    rf.persist()
}
```
### InstallSnapshot RPC
If a follower falls far behind (e.g., it was offline while the leader took a snapshot and discarded the logs the follower needs), standard `AppendEntries` cannot bring it up to date. In this case, the leader uses the InstallSnapshot RPC to send its entire state to the follower.

The follower must decide whether to replace its entire log or retain a suffix if it already has some of the data:
```go
func (rf *Raft) InstallSnapshot(args *InstallSnapshotArgs, reply *InstallSnapshotReply) {
	rf.mu.Lock()
	defer rf.mu.Unlock()

	// < snip - check if installing snapshot is valid >

  // Case 1: The snapshot covers more ground than our current log
	if rf.snapLastIndex+len(rf.logs) < args.LastIncludeIndex {
        // Discard entire log and reset
		rf.logs = make([]LogEntry, 0)
		rf.logs = append(rf.logs, LogEntry{Term: args.LastIncludeTerm})

	} else { 
    // Case 2: We have more recent entries than the snapshot; retain the suffix
		new_logs := make([]LogEntry, 0)
		new_logs = append(new_logs, LogEntry{Term: args.LastIncludeTerm})
        // Keep the logs that follow the snapshot index
		new_logs = append(new_logs, rf.logs[args.LastIncludeIndex-rf.snapLastIndex:]...)
		rf.logs = new_logs
	}
	
	// Installing a snapshot is effectively committing state
    // < snip - update metadata >
}

```

### Sharded KV on top of Raft (5A, 5B)

The ShardKV server wraps Raft to create a linearizable (every operation "appears" to happen instantaneously), key-value store. It submits operations (Get, Put, Append) to Raft and waits for them to be committed.

**Ensuring Linearizability**
We use a Submit-and-Wait pattern to guarantee strong consistency:

1. Request: Handler receives a client RPC.

2. Submit: Calls `rf.Start(op)` to append the operation to the Raft log.

3. Wait: Listens on a dedicated channel for that specific log index.

4. Execute: When the Raft loop applies the log, the result is sent back to the handler.

To handle network partitions and retries (at-most-once semantics), the system tracks the latest serialNumber seen for each `ClientId`.

```go
func (kv *ShardKV) PutAppend(args *PutAppendArgs, reply *PutAppendReply) {
	op := Op{
        Op: args.Op, Key: args.Key, Value: args.Value, 
        ClientId: args.ClientId, RequestId: args.RequestId // Deduplication ID
    }

  // Submit to Raft and wait for consensus
	ok, _ := kv.sendToRaftAndWait(op)

	if !ok {
		reply.Err = ErrWrongLeader
		return
	}
	reply.Err = OK
}
```
The system scales by partitioning data (sharding) across multiple replica groups. This requires two coordinated components:

### The Shard Controller

The `ShardCtrler` is a separate, fault-tolerant service (itself a Raft cluster) that manages the cluster configuration. It decides which replica group is responsible for which shards.

The controller exposes three main RPCs: `Join`, `Leave`, and `Move`.

-   **Join**: Adds new replica groups to the system.
-   **Leave**: Removes replica groups.
-   **Move**: Manually moves a shard from one group to another.

Each of these RPCs is committed through the controller's own Raft instance. When a command is applied, the controller generates a new, numbered configuration. The most critical logic is in `handleJoin` and `handleLeave`, which rebalance the shards as evenly as possible across the available groups.

```go
func (sc *ShardCtrler) handleJoin(op *Op) {
	latestConfig := sc.configs[len(sc.configs)-1]
    // ... complex logic to calculate the new shard distribution ...
    // ... by determining base load, remainders, and reassigning ...
    // ... shards from overloaded groups to new groups. ...

	newConfig := Config{Num: latestConfig.Num + 1, Shards: newShards, Groups: newGroups}
	sc.configs = append(sc.configs, newConfig)
}
```

This is the most complex part of the system. The `ShardKV` servers must periodically poll the `ShardCtrler` to see if there's a new configuration.

```go
func (kv *ShardKV) runConfigPolling() {
	for !kv.killed() {
		if kv.isLeader() {
			latestConfig := kv.sm.Query(-1)
            // If we are behind, process reconfiguration
			if latestConfig.Num > kv.config.Num {
                // Pull shard data from other groups
				transferShards := kv.getShardsFromOthers(latestConfig)
                
                // Commit reconfiguration + new data to Raft
				kv.sendToRaftAndWait(ReconfigureOp{Data: transferShards})
			}
		}
		time.Sleep(PollInterval)
	}
}
```

When a `ShardKV` leader detects a new configuration, it determines which shards it now owns but doesn't have the data for. It then issues a `TransferShard` RPC to the replica group that *used to* own that shard to pull the data.

Once the leader has gathered all the necessary data from other groups, it proposes a `Reconfigure` operation to its own Raft log. This operation contains the new configuration and all the shard data it just received. When this log entry is applied by the `runKVServer` loop, the replica group atomically updates to the new configuration and ingests the new data.

```markdown
And that's it!
```

For anyone interested in a deep, hands-on understanding of distributed systems, I cannot recommend the [MIT 6.824 course materials](https://pdos.csail.mit.edu/6.824/) highly enough. I think I forget to mention that the video lectures are also available on [Youtube](https://www.youtube.com/playlist?list=PLrw6a1wE39_tb2fErI4-WkMbsvGQk9_UB). You can pretty much get the full course experience 😁 - well, without the pain of mid-term and final exam !
