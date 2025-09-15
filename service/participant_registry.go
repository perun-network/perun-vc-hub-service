package service

import (
	"fmt"
	"log"
	"sync"

	"github.com/nervosnetwork/ckb-sdk-go/v2/types"
	"perun.network/perun-ckb-backend/wallet/address"
)

// ParticpantRegistry is an interface to manage the participants in the hub in a thread-safe manner.
type ParticipantRegistry interface {
	RegisterParticipant(addr string, part address.Participant) error

	IsParticipantInNetwork(part address.Participant, network types.Network) (bool, error)

	IsAddressInNetwork(addr string) (bool, error)

	GetAllParticipants() ([]address.Participant, error)

	RemoveParticipant(part address.Participant) error
}

type LocalParticipantRegistry struct {
	participants map[string]address.Participant
	mu           sync.RWMutex
}

func NewLocalParticipantRegistry() *LocalParticipantRegistry {
	return &LocalParticipantRegistry{
		participants: make(map[string]address.Participant),
	}
}

func (r *LocalParticipantRegistry) RegisterParticipant(addr string, part address.Participant) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.participants[addr] = part
	log.Printf("Registered participant: %s\n", addr)
	return nil
}

func (r *LocalParticipantRegistry) IsAddressInNetwork(addr string) (bool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	_, exists := r.participants[addr]
	return exists, nil
}

func (r *LocalParticipantRegistry) IsParticipantInNetwork(part address.Participant, network types.Network) (bool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	ckbAddr, err := part.ToCKBAddress(network).Encode()
	if err != nil {
		return false, fmt.Errorf("failed to convert participant to CKB address: %w", err)
	}
	_, exists := r.participants[ckbAddr]
	return exists, nil
}

func (r *LocalParticipantRegistry) GetAllParticipants() ([]address.Participant, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	participants := make([]address.Participant, 0, len(r.participants))
	for _, part := range r.participants {
		participants = append(participants, part)
	}
	return participants, nil
}

func (r *LocalParticipantRegistry) RemoveParticipant(part address.Participant) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	ckbAddr, err := part.ToCKBAddress(types.NetworkTest).Encode()
	if err != nil {
		return fmt.Errorf("failed to convert participant to CKB address: %w", err)
	}
	delete(r.participants, ckbAddr)
	return nil
}
