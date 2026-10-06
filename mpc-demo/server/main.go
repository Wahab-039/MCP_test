package main

import (
	"encoding/json"
	"fmt"
	"mpc-demo/mpc"
	"mpc-demo/proto"
	"net"
	"os"
	"sync"
)

const (
	address   = "localhost:9000"
	secret    = "HelloMPC"
	threshold = 2
)

var partyPINs = map[string]string{
	"A": "123",
	"B": "456",
	"C": "789",
}

var partyNames = []string{"A", "B", "C"}

// server holds all shared state. A single mutex guards auth, hub, and the
// threshold check so they are always atomic.
type server struct {
	session *mpc.Session
	mu      sync.Mutex
	hub     map[string]net.Conn // party name → open connection
	done    bool                // true once secret has been broadcast
}

func main() {
	printBanner()

	session, err := mpc.NewSession(secret, partyNames, threshold, partyPINs)
	if err != nil {
		fatalf("session init failed: %v", err)
	}

	fmt.Println("  Secret split into 3 shares (2-of-3 threshold)")
	fmt.Println("  Each share is PIN-protected")
	fmt.Println()

	ln, err := net.Listen("tcp", address)
	if err != nil {
		fatalf("failed to bind %s: %v", address, err)
	}
	defer ln.Close()

	fmt.Printf("  Listening on %s\n", address)
	fmt.Println("  Waiting for parties to connect...")
	fmt.Println()
	fmt.Println("  ──────────────────────────────────────────")

	srv := &server{
		session: session,
		hub:     make(map[string]net.Conn),
	}

	for {
		conn, err := ln.Accept()
		if err != nil {
			fmt.Printf("  [server] accept error: %v\n", err)
			continue
		}
		go srv.handleClient(conn)
	}
}

func (s *server) handleClient(conn net.Conn) {
	var req proto.AuthRequest
	if err := json.NewDecoder(conn).Decode(&req); err != nil {
		send(conn, proto.ServerMessage{Status: proto.StatusError, Message: "invalid request"})
		conn.Close()
		return
	}

	fmt.Printf("  [server] Party %s attempting to authenticate\n", req.Party)

	// Lock for the entire auth + hub registration + threshold check block.
	s.mu.Lock()

	if s.done {
		s.mu.Unlock()
		send(conn, proto.ServerMessage{Status: proto.StatusRejected, Message: "session already completed"})
		conn.Close()
		return
	}

	if !s.session.Authenticate(req.Party, req.PIN) {
		s.mu.Unlock()
		fmt.Printf("  [server] Party %s rejected\n", req.Party)
		send(conn, proto.ServerMessage{Status: proto.StatusRejected, Message: "wrong PIN or already authenticated"})
		conn.Close()
		return
	}

	// Register connection in hub while still holding the lock.
	s.hub[req.Party] = conn
	count := s.session.AuthenticatedCount()
	fmt.Printf("  [server] Party %s authenticated (%d/%d)\n", req.Party, count, s.session.Total)

	// Send waiting confirmation to this party.
	send(conn, proto.ServerMessage{
		Status:  proto.StatusWaiting,
		Message: fmt.Sprintf("Authenticated. Waiting for threshold (%d/%d).", count, s.session.Threshold),
	})

	// Check threshold — still inside the lock so no other goroutine can
	// sneak in between authentication and broadcast.
	if count >= s.session.Threshold {
		result, err := s.session.Compute()
		if err != nil || !result.ThresholdMet {
			s.mu.Unlock()
			return
		}

		s.done = true
		fmt.Printf("  [server] Threshold met! Broadcasting to %v\n", result.Participants)

		msg := proto.ServerMessage{
			Status:  proto.StatusUnlocked,
			Message: "Threshold satisfied. Secret reconstructed.",
			Secret:  result.Secret,
		}
		for party, c := range s.hub {
			if err := json.NewEncoder(c).Encode(msg); err != nil {
				fmt.Printf("  [server] failed to send to %s: %v\n", party, err)
			}
			c.Close()
		}
	}

	s.mu.Unlock()
}

func send(conn net.Conn, msg proto.ServerMessage) {
	json.NewEncoder(conn).Encode(msg)
}

func printBanner() {
	fmt.Println()
	fmt.Println("  ╔══════════════════════════════════════════╗")
	fmt.Println("  ║           MPC Server — 2-of-3            ║")
	fmt.Println("  ╚══════════════════════════════════════════╝")
	fmt.Println()
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "\n  ERROR: "+format+"\n\n", args...)
	os.Exit(1)
}
