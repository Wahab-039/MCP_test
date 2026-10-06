package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"mpc-demo/proto"
	"net"
	"os"
	"strings"
)

const address = "localhost:9000"

func main() {
	printBanner()

	scanner := bufio.NewScanner(os.Stdin)

	party := prompt(scanner, "  Your party name (A / B / C): ")
	pin := prompt(scanner, fmt.Sprintf("  PIN for party %s: ", strings.ToUpper(party)))

	fmt.Println("\n  Connecting to MPC server...")

	conn, err := net.Dial("tcp", address)
	if err != nil {
		fatalf("could not connect to server at %s: %v", address, err)
	}
	defer conn.Close()

	fmt.Println("  Connected.\n")

	req := proto.AuthRequest{
		Party: strings.ToUpper(party),
		PIN:   pin,
	}
	if err := json.NewEncoder(conn).Encode(req); err != nil {
		fatalf("failed to send credentials: %v", err)
	}

	fmt.Println("  ──────────────────────────────────────────")

	// Read messages from the server until the connection closes.
	decoder := json.NewDecoder(conn)
	for {
		var msg proto.ServerMessage
		if err := decoder.Decode(&msg); err != nil {
			// Connection closed by server after broadcast — normal exit.
			break
		}

		switch msg.Status {
		case proto.StatusRejected:
			fmt.Printf("\n  ✗ Rejected: %s\n\n", msg.Message)
			os.Exit(1)

		case proto.StatusWaiting:
			fmt.Printf("\n  ✓ %s\n", msg.Message)
			fmt.Println("  Waiting for other parties...\n")

		case proto.StatusUnlocked:
			fmt.Println("\n  ✓ Threshold satisfied")
			fmt.Println("  ✓ MPC computation completed")
			fmt.Printf("\n  ┌─────────────────────────────────┐\n")
			fmt.Printf("  │  Result: Secret unlocked        │\n")
			fmt.Printf("  │  Value : %-23s│\n", msg.Secret)
			fmt.Printf("  └─────────────────────────────────┘\n\n")
			return

		case proto.StatusError:
			fmt.Printf("\n  ✗ Server error: %s\n\n", msg.Message)
			os.Exit(1)
		}
	}
}

func prompt(scanner *bufio.Scanner, label string) string {
	fmt.Print(label)
	scanner.Scan()
	return strings.TrimSpace(scanner.Text())
}

func printBanner() {
	fmt.Println()
	fmt.Println("  ╔══════════════════════════════════════════╗")
	fmt.Println("  ║           MPC Client — 2-of-3            ║")
	fmt.Println("  ╚══════════════════════════════════════════╝")
	fmt.Println()
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "\n  ERROR: "+format+"\n\n", args...)
	os.Exit(1)
}
