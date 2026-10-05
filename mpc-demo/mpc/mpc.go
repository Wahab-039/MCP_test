package mpc

import (
	"errors"
	"fmt"
	"mpc-demo/splitting"
	"strings"
)

type Party struct {
	Name  string
	PIN   string
	Share splitting.Share
}

type Session struct {
	Parties   map[string]Party
	Threshold int
	Total     int
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
		upperName := strings.ToUpper(name)
		parties[upperName] = Party{
			Name:  upperName,
			PIN:   pins[upperName],
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

func (s *Session) Compute(participantNames []string) (Result, error) {
	seen := make(map[string]bool)
	var participants []string
	for _, name := range participantNames {
		upper := strings.ToUpper(name)
		if seen[upper] {
			continue
		}
		seen[upper] = true
		participants = append(participants, upper)
	}

	for _, name := range participants {
		if _, ok := s.Parties[name]; !ok {
			return Result{}, fmt.Errorf("unknown party: %q (registered: %s)", name, s.registeredNames())
		}
	}

	if len(participants) < s.Threshold {
		return Result{
			ThresholdMet: false,
			Participants: participants,
		}, nil
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

func (s *Session) registeredNames() string {
	names := make([]string, 0, len(s.Parties))
	for name := range s.Parties {
		names = append(names, name)
	}
	return strings.Join(names, ", ")
}
