package mpc

import (
	"errors"
	"fmt"
	"mpc-demo/splitting"
	"strings"
	"sync"
)

type Party struct {
	Name  string
	PIN   string
	Share splitting.Share
}

type Session struct {
	Parties       map[string]Party
	Threshold     int
	Total         int
	authenticated []string
	mu            sync.Mutex
}

type Result struct {
	ThresholdMet bool
	Secret       string
	Participants []string
}

func NewSession(secret string, partyNames []string, threshold int, pins map[string]string) (*Session, error) {
	n := len(partyNames)
	if threshold < 1 {
		return nil, errors.New("threshold must be at least 1")
	}
	if threshold > n {
		return nil, fmt.Errorf("threshold %d exceeds number of parties %d", threshold, n)
	}

	shares, err := splitting.Split(secret, n, threshold)
	if err != nil {
		return nil, fmt.Errorf("secret sharing failed: %w", err)
	}

	parties := make(map[string]Party, n)
	for i, name := range partyNames {
		upper := strings.ToUpper(name)
		parties[upper] = Party{
			Name:  upper,
			PIN:   pins[upper],
			Share: shares[i],
		}
	}

	return &Session{
		Parties:   parties,
		Threshold: threshold,
		Total:     n,
	}, nil
}

func (s *Session) VerifyPIN(partyName, pin string) bool {
	party, ok := s.Parties[strings.ToUpper(partyName)]
	if !ok {
		return false
	}
	return party.PIN == pin
}

// Authenticate marks a party as authenticated. Returns false if the party
// is unknown, already authenticated, or the PIN is wrong.
func (s *Session) Authenticate(partyName, pin string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	upper := strings.ToUpper(partyName)
	if !s.verifyPINUnsafe(upper, pin) {
		return false
	}
	for _, a := range s.authenticated {
		if a == upper {
			return false // already in
		}
	}
	s.authenticated = append(s.authenticated, upper)
	return true
}

// AuthenticatedCount returns how many parties have successfully authenticated.
func (s *Session) AuthenticatedCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.authenticated)
}

// Compute attempts reconstruction using all currently authenticated parties.
func (s *Session) Compute() (Result, error) {
	s.mu.Lock()
	participants := make([]string, len(s.authenticated))
	copy(participants, s.authenticated)
	s.mu.Unlock()

	if len(participants) < s.Threshold {
		return Result{ThresholdMet: false, Participants: participants}, nil
	}

	shares := make([]splitting.Share, 0, len(participants))
	for _, name := range participants {
		shares = append(shares, s.Parties[name].Share)
	}

	secret, err := splitting.Reconstruct(shares)
	if err != nil {
		return Result{}, fmt.Errorf("reconstruction failed: %w", err)
	}

	return Result{
		ThresholdMet: true,
		Secret:       secret,
		Participants: participants,
	}, nil
}

func (s *Session) verifyPINUnsafe(upper, pin string) bool {
	party, ok := s.Parties[upper]
	if !ok {
		return false
	}
	return party.PIN == pin
}

func (s *Session) registeredNames() string {
	names := make([]string, 0, len(s.Parties))
	for name := range s.Parties {
		names = append(names, name)
	}
	return strings.Join(names, ", ")
}
