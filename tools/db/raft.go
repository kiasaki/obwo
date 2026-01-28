// see https://notes.eatonphil.com/2023-05-25-raft.html
package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net"
	"net/http"
	"net/rpc"
	"os"
	"path"
	"sync"
	"time"

	olog "obwo/libraries/log"
)

type RPCMessage struct {
	Term uint64
}

type RequestVoteRequest struct {
	RPCMessage
	CandidateId  uint64
	LastLogIndex int64
	LastLogTerm  uint64
}

type RequestVoteResponse struct {
	RPCMessage
	VoteGranted bool
}

type AppendEntriesRequest struct {
	RPCMessage
	LeaderId     uint64
	PrevLogIndex uint64
	PrevLogTerm  uint64
	Entries      []Entry
	LeaderCommit uint64
}

type AppendEntriesResponse struct {
	RPCMessage
	Success bool
}

type ApplyRequest struct {
	RPCMessage
	Command []byte
}

type ApplyResponse struct {
	RPCMessage
	Result []byte
	Error  error
}

type StateMachine interface {
	Apply(cmd []byte) ([]byte, error)
}

type ApplyResult struct {
	Result []byte
	Error  error
}

type Entry struct {
	Id      int64
	Command []byte
	Term    uint64
	result  chan ApplyResult
}

type PersistentState struct {
	Log         []Entry
	VotedFor    uint64
	CurrentTerm uint64
}

type ClusterMember struct {
	Id         uint64
	Address    string
	nextIndex  uint64
	matchIndex uint64
	votedFor   uint64
	rpcClient  *rpc.Client
}

type ServerState string

const (
	leaderState    ServerState = "leader"
	followerState              = "follower"
	candidateState             = "candidate"
)

type Server struct {
	done    bool
	server  *http.Server
	mu      sync.Mutex
	Persist func(PersistentState)
	Restore func() PersistentState
	// persisted
	currentTerm uint64
	log         []Entry
	// readonly
	id               uint64
	address          string
	electionTimeout  time.Time
	heartbeatMs      int
	heartbeatTimeout time.Time
	statemachine     StateMachine
	metadataDir      string
	fd               *os.File
	// volatile
	commitIndex  uint64
	lastApplied  uint64
	state        ServerState
	cluster      []ClusterMember
	clusterIndex int
}

func NewServer(cluster []ClusterMember, statemachine StateMachine, metadataDir string, clusterIndex int) *Server {
	return &Server{
		id:           cluster[clusterIndex].Id,
		address:      cluster[clusterIndex].Address,
		cluster:      cluster,
		statemachine: statemachine,
		metadataDir:  metadataDir,
		clusterIndex: clusterIndex,
		heartbeatMs:  500,
		mu:           sync.Mutex{},
	}
}

func (s *Server) Start() {
	s.mu.Lock()
	s.state = followerState
	s.done = false
	s.restore()
	s.mu.Unlock()

	prefix := fmt.Sprintf("[db%d] ", s.id)
	log.Default().SetPrefix(prefix)
	olog.Setup(prefix)

	rpcServer := rpc.NewServer()
	rpcServer.Register(s)
	l, err := net.Listen("tcp", s.address)
	if err != nil {
		panic(err)
	}
	mux := http.NewServeMux()
	mux.Handle(rpc.DefaultRPCPath, rpcServer)

	s.server = &http.Server{Handler: mux}
	go s.server.Serve(l)

	go func() {
		s.mu.Lock()
		s.resetElectionTimeout()
		s.mu.Unlock()

		for {
			s.mu.Lock()
			if s.done {
				s.mu.Unlock()
				return
			}
			state := s.state
			s.mu.Unlock()
			switch state {
			case leaderState:
				s.heartbeat()
				s.advanceCommitIndex()
			case followerState:
				s.timeout()
				s.advanceCommitIndex()
			case candidateState:
				s.timeout()
				s.becomeLeader()
			}
		}
	}()
}

func (s *Server) HandleRequestVoteRequest(req RequestVoteRequest, rsp *RequestVoteResponse) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.updateTerm(req.RPCMessage)

	log.Printf("Received vote request from %d\n", req.CandidateId)
	rsp.VoteGranted = false
	rsp.Term = s.currentTerm

	if req.Term < s.currentTerm {
		log.Printf("Not granting vote request from %d\n", req.CandidateId)
		assert("VoteGranted = false", rsp.VoteGranted, false)
		return nil
	}
	lastLogTerm := s.log[len(s.log)-1].Term
	lastLogId := s.log[len(s.log)-1].Id
	logOk := req.LastLogTerm > lastLogTerm ||
		(req.LastLogTerm == lastLogTerm && req.LastLogIndex >= lastLogId)
	grant := req.Term == s.currentTerm &&
		logOk &&
		(s.getVotedFor() == 0 || s.getVotedFor() == req.CandidateId)
	if grant {
		log.Printf("Voted for %d\n", req.CandidateId)
		s.setVotedFor(req.CandidateId)
		rsp.VoteGranted = true
		s.resetElectionTimeout()
		s.persist()
	} else {
		log.Printf("Not granting vote request from %d\n", +req.CandidateId)
	}

	return nil
}

func (s *Server) heartbeat() {
	s.mu.Lock()
	defer s.mu.Unlock()
	timeForHeartbeat := time.Now().After(s.heartbeatTimeout)
	if timeForHeartbeat {
		s.heartbeatTimeout = time.Now().Add(time.Duration(s.heartbeatMs) * time.Millisecond)
		log.Println("Sending heartbeat")
		s.appendEntries()
	}
}

func (s *Server) appendEntries() {
	var MAX_APPEND_ENTRIES_BATCH = 8_000
	for i := range s.cluster {
		if i == s.clusterIndex {
			continue
		}

		go func(i int) {
			s.mu.Lock()
			next := s.cluster[i].nextIndex
			prevLogIndex := next - 1
			prevLogTerm := s.log[prevLogIndex].Term

			var entries []Entry
			if uint64(len(s.log)-1) >= s.cluster[i].nextIndex {
				//log.Printf("len: %d, next: %d, server: %d\n", len(s.log), next, s.cluster[i].Id)
				entries = s.log[next:]
			}
			if len(entries) > MAX_APPEND_ENTRIES_BATCH {
				entries = entries[:MAX_APPEND_ENTRIES_BATCH]
			}

			lenEntries := uint64(len(entries))
			req := AppendEntriesRequest{
				RPCMessage: RPCMessage{
					Term: s.currentTerm,
				},
				LeaderId:     s.cluster[s.clusterIndex].Id,
				PrevLogIndex: prevLogIndex,
				PrevLogTerm:  prevLogTerm,
				Entries:      entries,
				LeaderCommit: s.commitIndex,
			}
			s.mu.Unlock()

			var rsp AppendEntriesResponse
			log.Printf("Sending %d entries to %d for term %d\n", len(entries), s.cluster[i].Id, req.Term)
			ok := s.rpcCall(i, "Server.HandleAppendEntriesRequest", req, &rsp)
			if !ok {
				return // retry next tick
			}

			s.mu.Lock()
			defer s.mu.Unlock()
			if s.updateTerm(rsp.RPCMessage) {
				return
			}

			dropStaleResponse := rsp.Term != req.Term && s.state == leaderState
			if dropStaleResponse {
				return
			}

			if rsp.Success {
				prev := s.cluster[i].nextIndex
				s.cluster[i].nextIndex = max(req.PrevLogIndex+lenEntries+1, 1)
				s.cluster[i].matchIndex = s.cluster[i].nextIndex - 1
				log.Printf("Messages (%d) accepted for %d. Prev Index: %d, Next Index: %d, Match Index: %d\n", len(req.Entries), s.cluster[i].Id, prev, s.cluster[i].nextIndex, s.cluster[i].matchIndex)
			} else {
				s.cluster[i].nextIndex = max(s.cluster[i].nextIndex-1, 1)
				log.Printf("Forced to go back to %d for: %d\n", s.cluster[i].nextIndex, s.cluster[i].Id)
			}
		}(i)
	}
}

func (s *Server) HandleAppendEntriesRequest(req AppendEntriesRequest, rsp *AppendEntriesResponse) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.updateTerm(req.RPCMessage)
	if req.Term == s.currentTerm && s.state == candidateState {
		s.state = followerState
	}
	rsp.Term = s.currentTerm
	rsp.Success = false

	if s.state != followerState {
		log.Println("Non-follower cannot append entries.")
		return nil
	}
	if req.Term < s.currentTerm {
		log.Printf("Dropping request from old leader %d: term %d\n", req.LeaderId, req.Term)
		return nil
	}
	s.resetElectionTimeout()

	logLen := uint64(len(s.log))
	validPreviousLog := req.PrevLogIndex == 0 /* This is the induction step */ ||
		(req.PrevLogIndex < logLen &&
			s.log[req.PrevLogIndex].Term == req.PrevLogTerm)
	if !validPreviousLog {
		log.Println("Not a valid log.")
		return nil
	}

	next := req.PrevLogIndex + 1
	nNewEntries := 0

	for i := next; i < next+uint64(len(req.Entries)); i++ {
		e := req.Entries[i-next]
		if i >= uint64(cap(s.log)) {
			newTotal := next + uint64(len(req.Entries))
			newLog := make([]Entry, i, newTotal*2)
			copy(newLog, s.log)
			s.log = newLog
		}

		if i < uint64(len(s.log)) && s.log[i].Term != e.Term {
			prevCap := cap(s.log)
			s.log = s.log[:i] // on conflict, delete
			assert("Capacity remains the same while we truncated.", cap(s.log), prevCap)
		}

		olog.Log("appending %d: %s", len(s.log), string(e.Command))
		if i < uint64(len(s.log)) {
			assert("Existing log is the same as new log", s.log[i].Term, e.Term)
		} else {
			s.log = append(s.log, e)
			assert("Length is directly related to the index.", uint64(len(s.log)), i+1)
			nNewEntries++
		}
	}

	if req.LeaderCommit > s.commitIndex {
		s.commitIndex = min(req.LeaderCommit, uint64(len(s.log)-1))
	}
	s.persist()
	rsp.Success = true
	return nil
}

func (s *Server) Apply(command []byte) ([]byte, error) {
	if s.state != leaderState {
		var rsp ApplyResponse
		req := ApplyRequest{
			RPCMessage: RPCMessage{Term: s.currentTerm},
			Command:    command,
		}
		ok := s.rpcCall(int(s.getVotedFor()), "Server.HandleApplyRequest", req, &rsp)
		if !ok {
			return nil, errors.New("error contacting leader")
		}
		return rsp.Result, rsp.Error
	}
	s.mu.Lock()
	log.Println("processing new entry")
	ch := make(chan ApplyResult)
	s.log = append(s.log, Entry{
		Id:      s.log[len(s.log)-1].Id + 1,
		Term:    s.currentTerm,
		Command: command,
		result:  ch,
	})
	s.persist()
	log.Println("waiting to be applied")
	s.mu.Unlock()
	s.appendEntries()
	result := <-ch
	return result.Result, result.Error
}

func (s *Server) advanceCommitIndex() {
	s.mu.Lock()
	defer s.mu.Unlock()

	// leader can update commitIndex on quorum
	if s.state == leaderState {
		lastLogIndex := uint64(len(s.log) - 1)

		for i := lastLogIndex; i > s.commitIndex; i-- {
			quorum := len(s.cluster)/2 + 1

			for j := range s.cluster {
				if quorum == 0 {
					break
				}

				isLeader := j == s.clusterIndex
				if s.cluster[j].matchIndex >= i || isLeader {
					quorum--
				}
			}

			if quorum == 0 {
				s.commitIndex = i
				log.Printf("New commit index: %d\n", i)
				break
			}
		}
	}
	if s.lastApplied <= s.commitIndex {
		l := s.log[s.lastApplied]
		if len(l.Command) != 0 {
			olog.Log("applying %d: %s", s.lastApplied, string(l.Command))
			res, err := s.statemachine.Apply(l.Command)
			if l.result != nil {
				l.result <- ApplyResult{Result: res, Error: err}
			}
		}

		s.lastApplied++
	}
}

func (s *Server) HandleApplyRequest(req ApplyRequest, rsp *ApplyResponse) error {
	if s.state != leaderState {
		return errors.New("ApplyRequest sent to non leader")
	}
	result, err := s.Apply(req.Command)
	rsp.Result = result
	rsp.Error = err
	return nil
}

func (s *Server) becomeLeader() {
	s.mu.Lock()
	defer s.mu.Unlock()

	quorum := len(s.cluster)/2 + 1
	for i := range s.cluster {
		if s.cluster[i].votedFor == s.id && quorum > 0 {
			quorum--
		}
	}
	if quorum == 0 {
		for i := range s.cluster {
			s.cluster[i].nextIndex = uint64(len(s.log) + 1)
			s.cluster[i].matchIndex = 0
		}
		olog.Log("new leader")
		s.state = leaderState
		s.log = append(s.log, Entry{Term: s.currentTerm, Command: nil})
		s.persist()
		s.heartbeatTimeout = time.Now()
	}
}

func (s *Server) resetElectionTimeout() {
	interval := time.Duration(rand.Intn(s.heartbeatMs*2) + s.heartbeatMs*2)
	log.Printf("New interval: %s\n", interval*time.Millisecond)
	s.electionTimeout = time.Now().Add(interval * time.Millisecond)
}

func (s *Server) timeout() {
	s.mu.Lock()
	defer s.mu.Unlock()

	hasTimedOut := time.Now().After(s.electionTimeout)
	if hasTimedOut {
		log.Println("Timed out, starting new election.")
		s.state = candidateState
		s.currentTerm++
		for i := range s.cluster {
			if i == s.clusterIndex {
				s.cluster[i].votedFor = s.id
			} else {
				s.cluster[i].votedFor = 0
			}
		}

		s.resetElectionTimeout()
		s.persist()
		s.requestVote()
	}
}

func (s *Server) requestVote() {
	for i := range s.cluster {
		if i == s.clusterIndex {
			continue
		}

		go func(i int) {
			s.mu.Lock()
			lastLogIndex := s.log[len(s.log)-1].Id
			lastLogTerm := s.log[len(s.log)-1].Term
			olog.Log("request vote from %d", s.cluster[i].Id)

			req := RequestVoteRequest{
				RPCMessage: RPCMessage{
					Term: s.currentTerm,
				},
				CandidateId:  s.id,
				LastLogIndex: lastLogIndex,
				LastLogTerm:  lastLogTerm,
			}
			s.mu.Unlock()

			var rsp RequestVoteResponse
			ok := s.rpcCall(i, "Server.HandleRequestVoteRequest", req, &rsp)
			if !ok {
				return // retry later
			}
			s.mu.Lock()
			defer s.mu.Unlock()

			if s.updateTerm(rsp.RPCMessage) {
				return
			}

			dropStaleResponse := rsp.Term != req.Term
			if dropStaleResponse {
				return
			}

			if rsp.VoteGranted {
				olog.Log("vote granted by %d", s.cluster[i].Id)
				s.cluster[i].votedFor = s.id
			} else {
				olog.Log("vote not granted by %d", s.cluster[i].Id)
			}
		}(i)
	}
}

func (s *Server) updateTerm(msg RPCMessage) bool {
	transitioned := false
	if msg.Term > s.currentTerm {
		s.currentTerm = msg.Term
		s.state = followerState
		s.setVotedFor(0)
		transitioned = true
		olog.Log("now follower")
		s.resetElectionTimeout()
		s.persist()
	}
	return transitioned
}

func (s *Server) rpcCall(i int, name string, req, rsp any) bool {
	s.mu.Lock()
	c := s.cluster[i]
	var err error
	var rpcClient *rpc.Client = c.rpcClient
	if c.rpcClient == nil {
		c.rpcClient, err = rpc.DialHTTP("tcp", c.Address)
		rpcClient = c.rpcClient
	}
	s.mu.Unlock()
	if err == nil {
		err = rpcClient.Call(name, req, rsp)
	}
	if err != nil {
		log.Printf("error calling %s on %d: %s\n", name, c.Id, err)
	}
	return err == nil
}

func (s *Server) persist() {
	state := PersistentState{
		Log:         s.log,
		VotedFor:    s.getVotedFor(),
		CurrentTerm: s.currentTerm,
	}
	if s.Persist != nil {
		s.Persist(state)
		return
	}
	s.fd.Truncate(0)
	s.fd.Seek(0, 0)
	check(json.NewEncoder(s.fd).Encode(state))
	check(s.fd.Sync())
	log.Printf("presisted term:%d log:%d voted:%d\n", s.currentTerm, len(s.log), state.VotedFor)
}

func (s *Server) restore() {
	defer func() {
		if len(s.log) == 0 {
			s.log = append(s.log, Entry{})
		}
	}()
	var v PersistentState
	if s.Restore != nil {
		v = s.Restore()
		return
	} else {
		if s.fd == nil {
			var err error
			s.fd, err = os.OpenFile(path.Join(s.metadataDir, fmt.Sprintf("db_%d.dat", s.id)), os.O_SYNC|os.O_CREATE|os.O_RDWR, 0755)
			check(err)
		}
		s.fd.Seek(0, 0)
		err := json.NewDecoder(s.fd).Decode(&v)
		if err == io.EOF {
			return
		}
		check(err)
	}
	s.log = v.Log
	s.setVotedFor(v.VotedFor)
	s.currentTerm = v.CurrentTerm
}

func (s *Server) setVotedFor(id uint64) {
	for i := range s.cluster {
		if i == s.clusterIndex {
			s.cluster[i].votedFor = id
			return
		}
	}
}

func (s *Server) getVotedFor() uint64 {
	for i := range s.cluster {
		if i == s.clusterIndex {
			return s.cluster[i].votedFor
		}
	}
	panic("unreachable")
}

func assert[T comparable](msg string, a, b T) {
	if a != b {
		panic(fmt.Sprintf("%s: got a = %#v, b = %#v", msg, a, b))
	}
}

func check(err error) {
	if err != nil {
		panic(err)
	}
}
