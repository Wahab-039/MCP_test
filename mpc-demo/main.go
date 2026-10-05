package main

import (
	"bufio"
	"fmt"
	"mpc-demo/mpc"
	"os"
	"strings"
)

const (
	secret       = "HelloMPC"
	totalParties = 3
	threshold    = 2
)

var (
	partyNames = []string{"A", "B", "C"}
	partyPINs  = map[string]string{
		"A": "123",
		"B": "456",
		"C": "789",
	}
)

func main() {
	printBanner()

	fmt.Printf("  Parties   : %s\n", strings.Join(partyNames, ", "))
	fmt.Printf("  Threshold : %d-of-%d\n\n", threshold, totalParties)

	fmt.Println("  Splitting secret into shares...")
	session, err := mpc.NewSession(secret, partyNames, threshold, partyPINs)
	if err != nil {
		fatalf("Failed to initialise MPC session: %v", err)
	}
	fmt.Println("  ✓ Shares distributed to parties A, B, C")
	fmt.Println("  ✓ Each share is PIN-protected\n")

	printDivider()

	scanner := bufio.NewScanner(os.Stdin)

	for {
		var authenticated []string

		fmt.Println("\n  Enter participating parties one by one.")
		fmt.Println("  Type a party name (A / B / C), or 'done' to compute, 'q' to quit.\n")

		for {
			fmt.Print("  Party: ")
			if !scanner.Scan() {
				goto done
			}

			input := strings.TrimSpace(strings.ToUpper(scanner.Text()))

			if input == "Q" || input == "QUIT" {
				fmt.Println("\n  Goodbye.")
				return
			}
			if input == "DONE" || input == "" {
				break
			}

			if _, ok := session.Parties[input]; !ok {
				fmt.Printf("  ✗ Unknown party %q. Valid parties: A, B, C\n", input)
				continue
			}

			alreadyIn := false
			for _, p := range authenticated {
				if p == input {
					alreadyIn = true
					break
				}
			}
			if alreadyIn {
				fmt.Printf("  ✗ Party %s already added this round.\n", input)
				continue
			}

			fmt.Printf("  PIN for party %s: ", input)
			if !scanner.Scan() {
				goto done
			}
			pin := strings.TrimSpace(scanner.Text())

			if session.VerifyPIN(input, pin) {
				fmt.Printf("  ✓ Party %s authenticated\n", input)
				authenticated = append(authenticated, input)
			} else {
				fmt.Printf("  ✗ Wrong PIN for party %s — share rejected\n", input)
			}
		}

		if len(authenticated) == 0 {
			fmt.Println("\n  No parties authenticated. Try again.")
			printDivider()
			continue
		}

		fmt.Printf("\n  Authenticated parties: %s\n", strings.Join(authenticated, ", "))
		runComputation(session, authenticated)
		printDivider()
	}

done:
	if err := scanner.Err(); err != nil {
		fatalf("Input error: %v", err)
	}
}

func runComputation(session *mpc.Session, participantNames []string) {
	fmt.Println("\n  Running MPC...\n")

	result, err := session.Compute(participantNames)
	if err != nil {
		fmt.Printf("  ✗ Error: %v\n\n", err)
		return
	}

	if result.ThresholdMet {
		fmt.Printf("  ✓ Threshold satisfied  (%d/%d parties)\n", len(result.Participants), session.Total)
		fmt.Println("  ✓ MPC computation completed")
		fmt.Printf("\n  ┌─────────────────────────────────┐\n")
		fmt.Printf("  │  Result: Secret unlocked        │\n")
		fmt.Printf("  │  Value : %-23s│\n", result.Secret)
		fmt.Printf("  └─────────────────────────────────┘\n\n")
	} else {
		fmt.Printf("  ✗ Threshold not satisfied  (%d/%d parties, need %d)\n",
			len(result.Participants), session.Total, session.Threshold)
		fmt.Println("  ✗ Secret remains locked\n")
	}
}

func printBanner() {
	fmt.Println()
	fmt.Println("  ╔══════════════════════════════════════════╗")
	fmt.Println("  ║          MPC Secret Sharing Demo         ║")
	fmt.Println("  ║   Shamir's 2-of-3 Threshold Scheme       ║")
	fmt.Println("  ╚══════════════════════════════════════════╝")
	fmt.Println()
}

func printDivider() {
	fmt.Println("  ──────────────────────────────────────────")
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "\n  ERROR: "+format+"\n\n", args...)
	os.Exit(1)
}
