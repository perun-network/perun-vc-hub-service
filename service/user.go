package service

import (
	"context"
	"errors"
	"fmt"
	"log"
	"math/big"

	"github.com/nervosnetwork/ckb-sdk-go/v2/types"
	"perun.network/go-perun/channel"
	"perun.network/go-perun/client"
	gpwallet "perun.network/go-perun/wallet"
	"perun.network/go-perun/watcher"
	"perun.network/go-perun/wire"
	"perun.network/go-perun/wire/protobuf"

	"perun.network/perun-ckb-backend/wallet/address"

	"perun.network/vc-hub-service/protocol"

	chproto "perun.network/channel-service/rpc/proto"
)

// ErrChannelNotFound is returned when a channel with the specified ID is not found.
var ErrChannelNotFound = errors.New("channel not found")

// User represent the owner of this hub-service
// The owner can set the assets supported by this service
// The owner can set a Fee structure for the service
type User struct {
	supportedAssets []channel.Asset       // List of assets supported by this service
	feeStructure    protocol.FeeStructure // Fee structure for the service
	feeWatcher      protocol.Watcher      // Watcher to verify on-chain fees have been paid
	Participant     address.Participant
	PerunClient     *client.Client
	WireAddress     wire.Address
	wsc             chproto.WalletServiceClient
	Channels        map[channel.ID]*client.Channel // Active channels of the user
}

func NewUser(participant address.Participant, wAddr wire.Address, bus wire.Bus, funder channel.Funder, adjudicator channel.Adjudicator, wallet gpwallet.Wallet, watcher watcher.Watcher, wsc chproto.WalletServiceClient) (*User, error) {
	c, err := client.New(wAddr, bus, funder, adjudicator, wallet, watcher)
	if err != nil {
		return nil, err
	}
	u := &User{
		Participant: participant,
		PerunClient: c,
		WireAddress: wAddr,
		wsc:         wsc,
		Channels:    make(map[channel.ID]*client.Channel),
	}
	go c.Handle(u, u)
	return u, nil
}

// CloseChannel closes the channel with the specified ID.
func (u *User) CloseChannel(ctxt context.Context, id channel.ID) error {
	ch, ok := u.Channels[id]
	if !ok {
		return ErrChannelNotFound
	}
	// Finalize the channel to enable fast settlement.
	if !ch.State().IsFinal {
		err := ch.Update(ctxt, func(state *channel.State) {
			state.IsFinal = true
		})
		if err != nil {
			panic(err)
		}
	}

	// Settle concludes the channel and withdraws the funds.
	err := ch.Settle(ctxt, false)
	if err != nil {
		panic(err)
	}

	// Close frees up channel resources.
	_ = ch.Close()
	delete(u.Channels, id)
	return nil
}

// GetChannels returns the current state of all channels.
func (u *User) GetChannels() ([]channel.State, []channel.Index) {
	var states []channel.State
	var actorIndexes []channel.Index
	for _, ch := range u.Channels {
		states = append(states, *ch.State().Clone())
		actorIndexes = append(actorIndexes, ch.Idx())
	}
	return states, actorIndexes
}

func (u *User) GetSupportedAssets() []channel.Asset {
	return u.supportedAssets
}

func (u *User) GetFees(assetsToFunds map[channel.Asset][]*big.Int) (map[channel.Asset]string, error) {
	// panic("GetFees in user not implemented")
	fees := make(map[channel.Asset]string)
	for asset, funds := range assetsToFunds {
		fee, err := u.feeStructure.GetFee(asset, funds)
		if err != nil {
			return nil, err
		}
		fees[asset] = ShannonToCKByte(fee).Text('f', 8) // Convert fee to CKByte string with 8 decimal places
	}
	return fees, nil
}

// startWatching starts the dispute watcher for the specified channel.
func (u *User) startWatching(ch *client.Channel) {
	go func() {
		err := ch.Watch(u)
		if err != nil {
			fmt.Printf("Watcher returned with error: %v", err)
		}
	}()
}

// NotifyAllState notifies the wallet service about the new state of the channel.
func (u *User) NotifyAllState(_, to *channel.State) {
	pbNewState, err := protobuf.FromState(to.Clone())
	if err != nil {
		panic(fmt.Sprintf("unable to encode state: %v", err))
	}

	resp, err := u.wsc.UpdateNotification(context.TODO(), &chproto.UpdateNotificationRequest{
		State: pbNewState,
	})
	if err != nil {
		panic(fmt.Sprintf("unable to send update notification to wallet: %v", err))
	}
	if !resp.GetAccepted() {
		panic("wallet rejected update")
	}
}

func (u *User) verifyOnChainFees(alloc *channel.Allocation) bool {
	// for each asset in allocation, verify whether fees have been paid.
	for idx, asset := range alloc.Assets {
		if ok := u.feeWatcher.FeesPaidForAsset(asset, alloc.Balances[idx]); !ok {
			log.Println("Fees not paid for asset:", asset, "at index ", idx)
			return false
		}
	}
	return true
}

// checks if fees have been paid online and only then accepts the channel proposal
func (u *User) HandleProposal(proposal client.ChannelProposal, responder *client.ProposalResponder) {
	addr, err := u.Participant.ToCKBAddress(types.NetworkTest).Encode()
	if err != nil {
		panic(fmt.Sprintf("encoding participant addr: %v", err))
	}
	log.Printf("Handling channel proposal as user: %s", u.Participant)
	log.Printf("Handling channel proposal as user: %s", addr)

	lcp, ok := proposal.(*client.LedgerChannelProposalMsg) // HandleProposal should never receive a vc proposal
	if !ok {
		_ = responder.Reject(context.TODO(), "only ledger channel proposals are supported")
		return
	}
	log.Println("Verifying on-chain fees have been paid")
	if !u.verifyOnChainFees(lcp.Base().InitBals) {
		_ = responder.Reject(context.TODO(), "on-chain fees have not been paid")
		return
	}
	pLcp, err := protobuf.FromLedgerChannelProposalMsg(lcp)
	if err != nil {
		_ = responder.Reject(context.TODO(), fmt.Sprintf("unable to encode proposal: %v", err))
		return
	}

	log.Println("Requesting nonce share from wallet")
	resp, err := u.wsc.OpenChannel(context.TODO(), &chproto.OpenChannelRequest{Proposal: pLcp.LedgerChannelProposalMsg})
	if err != nil {
		_ = responder.Reject(context.TODO(), fmt.Sprintf("unable to open channel: %v", err))
		return
	}
	log.Println("Received nonce share from wallet")
	ns := resp.GetNonceShare()
	if ns == nil {
		if resp.GetRejected() != nil {
			_ = responder.Reject(context.TODO(), resp.GetRejected().GetReason())
			return
		} else {
			_ = responder.Reject(context.TODO(), "wallet rejected channel proposal")
			return
		}
	}
	nonceShare := client.NonceShare{}
	copy(nonceShare[:], ns)
	cpa := client.LedgerChannelProposalAccMsg{
		BaseChannelProposalAcc: client.BaseChannelProposalAcc{
			ProposalID: lcp.ProposalID,
			NonceShare: nonceShare,
		},
		Participant: &u.Participant,
	}
	ch, err := responder.Accept(context.TODO(), &cpa)
	if err != nil {
		panic(err)
	}
	u.Channels[ch.ID()] = ch
	u.startWatching(ch)
	ch.OnUpdate(u.NotifyAllState)
	u.NotifyAllState(nil, ch.State())
}

func (u *User) HandleUpdate(_ *channel.State, update client.ChannelUpdate, responder *client.UpdateResponder) {
	//a hub service should never recive an update unless we have a recurring fee model
	//TODO: implement this when we have a recurring fee model
	_ = responder.Reject(context.TODO(), "channel updates are not supported")
}

// HandleAdjudicatorEvent handles an adjudicator event.
func (u *User) HandleAdjudicatorEvent(event channel.AdjudicatorEvent) {
	// TODO:
	log.Printf("Adjudicator event: type = %T\n", event)
}
