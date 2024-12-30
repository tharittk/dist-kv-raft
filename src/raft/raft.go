package raft

import (
	"bytes"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"

	"6.5840/labgob"
	"6.5840/labrpc"
)

type ApplyMsg struct {
	CommandValid bool
	Command      interface{}
	CommandIndex int

	SnapshotValid bool
	Snapshot      []byte
	SnapshotTerm  int
	SnapshotIndex int
}

type LogEntry struct {
	Term    int
	Command interface{}
}

type State string

const (
	Leader    State = "Leader"
	Candidate       = "Candidate"
	Follower        = "Follower"
)

// A Go object implementing a single Raft peer.
type Raft struct {
	mu        sync.Mutex          // Lock to protect shared access to this peer's state
	peers     []*labrpc.ClientEnd // RPC end points of all peers
	persister *Persister          // Object to hold this peer's persisted state
	me        int                 // this peer's index into peers[]
	dead      int32               // set by Kill()

	// persistent state on all servers
	currentTerm int
	votedFor    int
	logs        []LogEntry

	// volatile state on all servers
	commitIndex int
	lastApplied int

	// volatile state on leaders
	nextIndex  []int
	matchIndex []int

	// other auxiliary states
	state       State
	voteCount   int
	applyCh     chan ApplyMsg
	winElectCh  chan bool
	stepDownCh  chan bool
	grantVoteCh chan bool
	heartbeatCh chan bool

	snapLastIndex int
	snapLastTerm  int
}

// return currentTerm and whether this server
// believes it is the leader.
func (rf *Raft) GetState() (int, bool) {
	rf.mu.Lock()
	defer rf.mu.Unlock()
	// Code hidden
}

// save Raft's persistent state to stable storage,
// where it can later be retrieved after a crash and restart.
// see paper's Figure 2 for a description of what should be persistent.
// lock must be held before calling this.
func (rf *Raft) persist() {
	w := new(bytes.Buffer)
	e := labgob.NewEncoder(w)
	// Code hidden

}

// restore previously persisted state.
func (rf *Raft) readPersist(data []byte) {
	if data == nil || len(data) < 1 {
		return
	}
	r := bytes.NewBuffer(data)
	d := labgob.NewDecoder(r)
	// Code hidden

}

func (rf *Raft) GetRaftStateSize() int {
	return rf.persister.RaftStateSize()
}

func (rf *Raft) ReadSnapshot() []byte {
	return rf.persister.ReadSnapshot()
}

func (rf *Raft) restoreFromSnapshot(data []byte) {
	if data == nil || len(data) < 1 {
		return
	}

	r := bytes.NewBuffer(data)
	d := labgob.NewDecoder(r)

	// Code hidden

}

// the service says it has created a snapshot that has
// all info up to and including index. this means the
// service no longer needs the log through (and including)
// that index. Raft should now trim its log as much len(as possible.
func (rf *Raft) Snapshot(index int, snapshot []byte) {

	// Code hidden

	// save for RPC before replace with zero

	// Code hidden

	// store snapshot

	// Code hidden

	// reset next index for new log offset

	// Code hidden

}

type InstallSnapshotArgs struct {
	Term             int
	LeaderId         int
	LastIncludeIndex int
	LastIncludeTerm  int
	Data             []byte
}

type InstallSnapshotReply struct {
	Term int
}

func (rf *Raft) InstallSnapshot(args *InstallSnapshotArgs, reply *InstallSnapshotReply) {
	// Code hidden

	// total log is shorter

	// Code hidden

	// or retain what follows

	// Code hidden

	// Installing snapshot is effectively commiting

	// Code hidden

}

// must acquire lock before calling
func (rf *Raft) sendInstallSnapshot(server int) {

	// Code hidden

	// re-try until Ok

	// Code hidden

}

// RequestVote RPC arguments structure.
type RequestVoteArgs struct {
	Term         int
	CandidateId  int
	LastLogIndex int
	LastLogTerm  int
}

// RequestVote RPC reply structure.
type RequestVoteReply struct {
	Term        int
	VoteGranted bool
}

// AppendEntries RPC arguments structure.
type AppendEntriesArgs struct {
	Term         int
	LeaderId     int
	PrevLogIndex int
	PrevLogTerm  int
	LeaderCommit int
	Entries      []LogEntry
}

// AppendEntries RPC reply structure.
type AppendEntriesReply struct {
	Term          int
	Success       bool
	ConflictIndex int
	ConflictTerm  int
}

// get the index of the last log entry.
// lock must be held before calling this.
func (rf *Raft) getLastIndex() int {
	return len(rf.logs) - 1 + rf.snapLastIndex
}

// get the term of the last log entry.
// lock must be held before calling this.
func (rf *Raft) getLastTerm() int {
	return rf.logs[rf.getLastIndex()-rf.snapLastIndex].Term
}

// get the randomized election timeout.
func (rf *Raft) getElectionTimeout() time.Duration {
	//return time.Duration(360 + rand.Intn(240))
	return time.Duration(120 + rand.Intn(40))
}

// send value to an un-buffered channel without blocking
func (rf *Raft) sendToChannel(ch chan bool, value bool) {
	select {
	case ch <- value:
	default:
	}
}

// step down to follower when getting higher term,
// lock must be held before calling this.
func (rf *Raft) stepDownToFollower(term int) {
	// Code hidden

	// step down if not follower, this check is needed
	// to prevent race where state is already follower

}

// check if the candidate's log is at least as up-to-date as ours
// lock must be held before calling this.
func (rf *Raft) isLogUpToDate(cLastIndex int, cLastTerm int) bool {
	// Code hidden
}

func (rf *Raft) applySnapshot() {

	rf.applyCh <- ApplyMsg{
		SnapshotValid: true,
		Snapshot:      rf.persister.ReadSnapshot(),
		SnapshotTerm:  rf.snapLastTerm,
		SnapshotIndex: rf.snapLastIndex,
	}

}

func (rf *Raft) applyLogs() {
	rf.mu.Lock()
	logsToApply := make([]ApplyMsg, 0)

	for i := rf.lastApplied + 1; i <= rf.commitIndex; i++ {
		logsToApply = append(logsToApply, ApplyMsg{
			CommandValid: true,
			Command:      rf.logs[i-rf.snapLastIndex].Command,
			CommandIndex: i,
		})
		rf.lastApplied = i
	}
	rf.mu.Unlock() // Release the lock before interacting with applyCh

	for _, msg := range logsToApply {
		rf.applyCh <- msg
	}
}

// RequestVote RPC handler.
func (rf *Raft) RequestVote(args *RequestVoteArgs, reply *RequestVoteReply) {
	// Code hidden
}

func (rf *Raft) sendRequestVote(server int, args *RequestVoteArgs, reply *RequestVoteReply) {
	ok := rf.peers[server].Call("Raft.RequestVote", args, reply)

	if !ok {
		return
	}

	rf.mu.Lock()
	defer rf.mu.Unlock()

	if rf.state != Candidate || args.Term != rf.currentTerm || reply.Term < rf.currentTerm {
		return
	}

	if reply.Term > rf.currentTerm {
		rf.stepDownToFollower(args.Term)
		rf.persist()
		return
	}

	if reply.VoteGranted {
		rf.voteCount++
		// only send once when vote count just reaches majority
		if rf.voteCount == len(rf.peers)/2+1 {
			rf.sendToChannel(rf.winElectCh, true)
		}
	}

}

// broadcast RequestVote RPCs to all peers in parallel.
// lock must be held before calling this.
func (rf *Raft) broadcastRequestVote() {
	// Code hidden
}

// AppendEntries RPC handler.
func (rf *Raft) AppendEntries(args *AppendEntriesArgs, reply *AppendEntriesReply) {

	// Code hidden

	// follower log is shorter than leader

	// Code hidden

	// log consistency check fails, i.e. different term at prevLogIndex

	// Code hidden

	// only truncate log if an existing entry conflicts with a new one

	// Code hidden

	// update commit index to min(leaderCommit, lastIndex)

	// Code hidden

}

// send a AppendEntries RPC to a server.
func (rf *Raft) sendAppendEntries(server int, args *AppendEntriesArgs, reply *AppendEntriesReply) {
	// Code hidden

	// update matchIndex and nextIndex of the follower
	if reply.Success {
		// match index should not regress in case of stale rpc response
		// Code hidden

	} else if reply.ConflictIndex < rf.snapLastIndex { // Need InstallSnapshot

		// Code hidden

	} else if reply.ConflictTerm < 0 {
		// follower's log shorter than leader's

		// Code hidden

	} else {
		// try to find the conflictTerm in log
		// Code hidden

		// if not found, set nextIndex to conflictIndex
		// Code hidden

	}

	// if there exists an N such that N > commitIndex, a majority of
	// matchIndex[i] >= N, and log[N].term == currentTerm, set commitIndex = N

	// Code hidden

}

// broadcast AppendEntries RPCs to all peers in parallel.
// lock must be held before calling this.
func (rf *Raft) broadcastAppendEntries() {
	// Code hidden
}

// the service using Raft (e.g. a k/v server) wants to start
// agreement on the next command to be appended to Raft's log. if this
// server isn't the leader, returns false. otherwise start the
// agreement and return immediately. there is no guarantee that this
// command will ever be committed to the Raft log, since the leader
// may fail or lose an election. even if the Raft instance has been killed,
// this function should return gracefully.
//
// the first return value is the index that the command will appear at
// if it's ever committed. the second return value is the current
// term. the third return value is true if this server believes it is
// the leader.
func (rf *Raft) Start(command interface{}) (int, int, bool) {
	// Code hidden
}

func (rf *Raft) Kill() {
	atomic.StoreInt32(&rf.dead, 1)
	// Your code here, if desired.
}

func (rf *Raft) killed() bool {
	z := atomic.LoadInt32(&rf.dead)
	return z == 1
}

// convert the raft state to leader.
func (rf *Raft) convertToLeader() {
	// Code hidden
}

// convert the raft state to candidate.
func (rf *Raft) convertToCandidate(fromState State) {
	// Code hidden
}

// reset the channels, needed when converting server state.
// lock must be held before calling this.
func (rf *Raft) resetChannels() {
	rf.winElectCh = make(chan bool)
	rf.stepDownCh = make(chan bool)
	rf.grantVoteCh = make(chan bool)
	rf.heartbeatCh = make(chan bool)
}

// main server loop.
func (rf *Raft) runServer() {
	for !rf.killed() {
		rf.mu.Lock()
		state := rf.state
		rf.mu.Unlock()
		switch state {
		case Leader:
			select {
			case <-rf.stepDownCh:
				// state should already be follower
			case <-time.After(30 * time.Millisecond):
				rf.mu.Lock()
				rf.broadcastAppendEntries()
				rf.mu.Unlock()
			}
		case Follower:
			select {
			case <-rf.grantVoteCh:
			case <-rf.heartbeatCh:
			case <-time.After(rf.getElectionTimeout() * time.Millisecond):
				rf.convertToCandidate(Follower)
			}
		case Candidate:
			select {
			case <-rf.stepDownCh:
				// state should already be follower
			case <-rf.winElectCh:
				rf.convertToLeader()
			case <-time.After(rf.getElectionTimeout() * time.Millisecond):
				rf.convertToCandidate(Candidate)
			}
		}
	}
}

// the service or tester wants to create a Raft server. the ports
// of all the Raft servers (including this one) are in peers[]. this
// server's port is peers[me]. all the servers' peers[] arrays
// have the same order. persister is a place for this server to
// save its persistent state, and also initially holds the most
// recent saved state, if any. applyCh is a channel on which the
// tester or service expects Raft to send ApplyMsg messages.
// Make() must return quickly, so it should start goroutines
// for any long-running work.
func Make(peers []*labrpc.ClientEnd, me int,
	persister *Persister, applyCh chan ApplyMsg) *Raft {
	rf := &Raft{}
	rf.peers = peers
	rf.persister = persister
	rf.me = me

	// Your initialization code here (2A, 2B, 2C).

	// Code hidden

	// initialize from state persisted before a crash

	// start the background server loop

}
