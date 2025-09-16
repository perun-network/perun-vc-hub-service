// Copyright 2024 PolyCrypt GmbH
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package client

import (
	"context"
	"fmt"
	"log"

	"perun.network/go-perun/channel"
	"perun.network/go-perun/client"
)

// HandleProposal is the callback for incoming channel proposals.
func (p *PaymentClient) HandleProposal(prop client.ChannelProposal, r *client.ProposalResponder) {
	// log.Println("Handling channel proposal for ", p.Name)
	// lcp, err := func() (*client.LedgerChannelProposalMsg, error) {
	// 	// Ensure that we got a ledger channel proposal.
	// 	lcp, ok := prop.(*client.LedgerChannelProposalMsg)
	// 	if !ok {
	// 		return nil, fmt.Errorf("invalid proposal type: %T", p)
	// 	}

	// 	// Check that we have the correct number of participants.
	// 	if lcp.NumPeers() != 2 {
	// 		return nil, fmt.Errorf("invalid number of participants: %d", lcp.NumPeers())
	// 	}
	// 	// Check that the channel has the expected assets and funding balances.
	// 	for i, assetAlloc := range lcp.FundingAgreement {
	// 		if assetAlloc[0].Cmp(assetAlloc[1]) != 0 {
	// 			return nil, fmt.Errorf("invalid funding balance for asset %d: %v", i, assetAlloc)
	// 		}

	// 	}
	// 	return lcp, nil
	// }()
	// if err != nil {
	// 	_ = r.Reject(context.TODO(), err.Error())
	// }

	// // Create a channel accept message and send it.
	// accept := lcp.Accept(
	// 	p.WalletAddress(),        // The Account we use in the channel.
	// 	client.WithRandomNonce(), // Our share of the channel nonce.
	// )
	// ch, err := r.Accept(context.TODO(), accept)
	// if err != nil {
	// 	log.Printf("Error accepting channel proposal: %v", err)
	// 	return
	// }

	// // Start the on-chain event watcher. It automatically handles disputes.
	// p.startWatching(ch)

	// // Store channel.
	// p.channels <- newPaymentChannel(ch, lcp.InitBals.Clone().Assets)
	// p.AcceptedChannel()

	switch proposal := prop.(type) {
	case *client.LedgerChannelProposalMsg:
		p.handleLedgerProposal(proposal, r)
	case *client.VirtualChannelProposalMsg:
		p.handleVirtualChannelProposal(proposal, r)
	default:
		_ = r.Reject(context.TODO(), fmt.Sprintf("invalid proposal type: %T", p))

	}
}

func (p *PaymentClient) handleLedgerProposal(prop *client.LedgerChannelProposalMsg, r *client.ProposalResponder) {
	// Check that we have the correct number of participants.
	log.Println("Handling ledger channel proposal for ", p.Name)
	if prop.NumPeers() != 2 {
		_ = r.Reject(context.TODO(), fmt.Sprintf("invalid number of participants: %d", prop.NumPeers()))
		return
	}
	// Check that the channel has the expected assets and funding balances.
	for i, assetAlloc := range prop.FundingAgreement {
		if assetAlloc[0].Cmp(assetAlloc[1]) != 0 {
			_ = r.Reject(context.TODO(), fmt.Sprintf("invalid funding balance for asset %d: %v", i, assetAlloc))
			return
		}

	}
	accept := prop.Accept(
		p.WalletAddress(),        // The Account we use in the channel.
		client.WithRandomNonce(), // Our share of the channel nonce.
	)
	ch, err := r.Accept(context.TODO(), accept)
	if err != nil {
		log.Printf("Error accepting channel proposal: %v", err)
	}

	// Start the on-chain event watcher. It automatically handles disputes.
	p.startWatching(ch)

	// Store channel.
	p.channels <- newPaymentChannel(ch, prop.InitBals.Clone().Assets)
}

func (p *PaymentClient) handleVirtualChannelProposal(prop *client.VirtualChannelProposalMsg, r *client.ProposalResponder) {
	log.Println("Handling virtual channel proposal for ", p.Name)
	if prop.NumPeers() != 2 {
		_ = r.Reject(context.TODO(), fmt.Sprintf("invalid number of participants: %d", prop.NumPeers()))
		return
	}

	accept := prop.Accept(
		p.WalletAddress(),        // The Account we use in the channel.
		client.WithRandomNonce(), // Our share of the channel nonce.
	)
	ch, err := r.Accept(context.TODO(), accept)
	if err != nil {
		// log.Errorf("error accepting channel proposal: %v", err)
		log.Printf("Error accepting channel proposal: %v", err)
	}

	// Start the on-chain event watcher. It automatically handles disputes.
	p.startWatching(ch)

	// Store channel.
	p.channels <- newPaymentChannel(ch, prop.InitBals.Clone().Assets)
}

// HandleUpdate is the callback for incoming channel updates.
func (p *PaymentClient) HandleUpdate(cur *channel.State, next client.ChannelUpdate, r *client.UpdateResponder) {
	// We accept every update that increases our balance.
	// err := func() error {
	// 	err := channel.AssertAssetsEqual(cur.Assets, next.State.Assets)
	// 	if err != nil {
	// 		return fmt.Errorf("invalid assets: %v", err)
	// 	}

	// 	receiverIdx := 1 - next.ActorIdx // This works because we are in a two-party channel.
	// 	for _, a := range cur.Assets {
	// 		curBal := cur.Allocation.Balance(receiverIdx, a)
	// 		nextBal := next.State.Allocation.Balance(receiverIdx, a)
	// 		if nextBal.Cmp(curBal) < 0 {
	// 			return fmt.Errorf("invalid balance: %v", nextBal)
	// 		}
	// 	}

	// 	return nil
	// }()
	// if next.State.IsFinal {
	// 	// always accept final state
	// 	err := r.Accept(context.TODO())
	// 	if err != nil {
	// 		panic(err)
	// 	}
	// 	return
	// }
	// err := acceptUpdateWithHigherBalances(cur, next)
	// if err != nil {
	// 	_ = r.Reject(context.TODO(), err.Error())
	// }

	// Send the acceptance message.
	log.Println(p.Name, "received update for channel ", cur.ID)
	err := r.Accept(context.TODO())
	if err != nil {
		panic(err)
	}
}

// func acceptUpdateWithHigherBalances(cur *channel.State, next client.ChannelUpdate) error {
// 	err := channel.AssertAssetsEqual(cur.Assets, next.State.Assets)
// 	if err != nil {
// 		return fmt.Errorf("invalid assets: %v", err)
// 	}

// 	receiverIdx := 1 - next.ActorIdx // This works because we are in a two-party channel.
// 	for _, a := range cur.Assets {
// 		curBal := cur.Allocation.Balance(receiverIdx, a)
// 		nextBal := next.State.Allocation.Balance(receiverIdx, a)
// 		if nextBal.Cmp(curBal) < 0 {
// 			return fmt.Errorf("invalid balance: %v", nextBal)
// 		}
// 	}

// 	return nil
// }

// HandleAdjudicatorEvent is the callback for smart contract events.
func (p *PaymentClient) HandleAdjudicatorEvent(e channel.AdjudicatorEvent) {
	log.Printf("Adjudicator event: type = %T, client = %v", e, p.Account)
}
