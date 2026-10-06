package main

import (
	"encoding/json"
	"mpc-demo/mpc"
	"mpc-demo/proto"
	"net"
	"sync"
	"testing"
	"time"
)

func startTestServer(t *testing.T) (addr string, cleanup func()) {
	return startTestServerWithTimeout(t, 0)
}

func startTestServerWithTimeout(t *testing.T, timeout time.Duration) (addr string, cleanup func()) {
	t.Helper()

	session, err := mpc.NewSession("HelloMPC", []string{"A", "B", "C"}, 2, map[string]string{
		"A": "123", "B": "456", "C": "789",
	})
	if err != nil {
		t.Fatalf("session: %v", err)
	}

	ln, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}

	srv := &server{
		session: session,
		hub:     make(map[string]net.Conn),
		timeout: timeout,
	}
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go srv.handleClient(conn)
		}
	}()

	return ln.Addr().String(), func() { ln.Close() }
}

func clientAuth(t *testing.T, addr, party, pin string) proto.ServerMessage {
	t.Helper()
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	json.NewEncoder(conn).Encode(proto.AuthRequest{Party: party, PIN: pin})

	var last proto.ServerMessage
	dec := json.NewDecoder(conn)
	for {
		conn.SetReadDeadline(time.Now().Add(2 * time.Second))
		var msg proto.ServerMessage
		if err := dec.Decode(&msg); err != nil {
			break
		}
		last = msg
		if msg.Status == proto.StatusUnlocked || msg.Status == proto.StatusRejected || msg.Status == proto.StatusTimeout || msg.Status == proto.StatusError {
			break
		}
	}
	return last
}

func TestThresholdMet(t *testing.T) {
	addr, cleanup := startTestServer(t)
	defer cleanup()

	var wg sync.WaitGroup
	results := make([]proto.ServerMessage, 2)

	wg.Add(2)
	go func() { defer wg.Done(); results[0] = clientAuth(t, addr, "A", "123") }()
	go func() { defer wg.Done(); results[1] = clientAuth(t, addr, "B", "456") }()
	wg.Wait()

	for i, r := range results {
		if r.Status != proto.StatusUnlocked {
			t.Errorf("client %d: expected unlocked, got %q (msg: %s)", i, r.Status, r.Message)
		}
		if r.Secret != "HelloMPC" {
			t.Errorf("client %d: expected secret HelloMPC, got %q", i, r.Secret)
		}
	}
}

func TestWrongPIN(t *testing.T) {
	addr, cleanup := startTestServer(t)
	defer cleanup()

	r := clientAuth(t, addr, "A", "wrongpin")
	if r.Status != proto.StatusRejected {
		t.Errorf("expected rejected, got %q", r.Status)
	}
}

func TestThresholdNotMet(t *testing.T) {
	addr, cleanup := startTestServer(t)
	defer cleanup()

	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	json.NewEncoder(conn).Encode(proto.AuthRequest{Party: "C", PIN: "789"})

	var msg proto.ServerMessage
	conn.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
	json.NewDecoder(conn).Decode(&msg)

	if msg.Status != proto.StatusWaiting {
		t.Errorf("expected waiting, got %q", msg.Status)
	}
}

func TestAllThreeParties(t *testing.T) {
	addr, cleanup := startTestServer(t)
	defer cleanup()

	var wg sync.WaitGroup
	results := make([]proto.ServerMessage, 3)

	wg.Add(3)
	go func() { defer wg.Done(); results[0] = clientAuth(t, addr, "A", "123") }()
	go func() { defer wg.Done(); results[1] = clientAuth(t, addr, "B", "456") }()
	go func() { defer wg.Done(); results[2] = clientAuth(t, addr, "C", "789") }()
	wg.Wait()

	unlocked := 0
	for _, r := range results {
		if r.Status == proto.StatusUnlocked && r.Secret == "HelloMPC" {
			unlocked++
		}
	}
	// At least 2 must receive the unlock (the 3rd might be waiting when broadcast fires)
	if unlocked < 2 {
		t.Errorf("expected at least 2 clients to receive unlock, got %d", unlocked)
	}
}

func TestSessionTimeout(t *testing.T) {
	addr, cleanup := startTestServerWithTimeout(t, 500*time.Millisecond)
	defer cleanup()

	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	json.NewEncoder(conn).Encode(proto.AuthRequest{Party: "A", PIN: "123"})

	dec := json.NewDecoder(conn)

	var msg1 proto.ServerMessage
	conn.SetReadDeadline(time.Now().Add(1 * time.Second))
	if err := dec.Decode(&msg1); err != nil {
		t.Fatalf("read first message: %v", err)
	}
	if msg1.Status != proto.StatusWaiting {
		t.Errorf("expected waiting, got %q", msg1.Status)
	}

	var msg2 proto.ServerMessage
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	if err := dec.Decode(&msg2); err != nil {
		t.Fatalf("read timeout message: %v", err)
	}
	if msg2.Status != proto.StatusTimeout {
		t.Errorf("expected timeout, got %q (message: %s)", msg2.Status, msg2.Message)
	}
}

func TestTimeoutThenNewSession(t *testing.T) {
	addr, cleanup := startTestServerWithTimeout(t, 300*time.Millisecond)
	defer cleanup()

	conn1, _ := net.Dial("tcp", addr)
	json.NewEncoder(conn1).Encode(proto.AuthRequest{Party: "A", PIN: "123"})

	dec1 := json.NewDecoder(conn1)
	var msg proto.ServerMessage
	dec1.Decode(&msg)
	dec1.Decode(&msg)
	conn1.Close()

	if msg.Status != proto.StatusTimeout {
		t.Errorf("first session: expected timeout, got %q", msg.Status)
	}

	time.Sleep(400 * time.Millisecond)

	var wg sync.WaitGroup
	results := make([]proto.ServerMessage, 2)

	wg.Add(2)
	go func() { defer wg.Done(); results[0] = clientAuth(t, addr, "A", "123") }()
	go func() { defer wg.Done(); results[1] = clientAuth(t, addr, "B", "456") }()
	wg.Wait()

	for i, r := range results {
		if r.Status != proto.StatusUnlocked {
			t.Errorf("second session client %d: expected unlocked, got %q", i, r.Status)
		}
	}
}
