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
	"math/big"
	"math/rand"
	"time"

	"github.com/decred/dcrd/dcrec/secp256k1/v4"
	"github.com/nervosnetwork/ckb-sdk-go/v2/rpc"
	"github.com/nervosnetwork/ckb-sdk-go/v2/types"
	"github.com/perun-network/perun-libp2p-wire/p2p"

	gpchannel "perun.network/go-perun/channel"
	"perun.network/go-perun/client"
	gpwallet "perun.network/go-perun/wallet"
	"perun.network/go-perun/watcher/local"
	"perun.network/go-perun/wire"
	"perun.network/perun-ckb-backend/backend"
	"perun.network/perun-ckb-backend/channel/adjudicator"
	"perun.network/perun-ckb-backend/channel/asset"
	"perun.network/perun-ckb-backend/channel/funder"
	ckbclient "perun.network/perun-ckb-backend/client"
	"perun.network/perun-ckb-backend/wallet"
	"perun.network/perun-ckb-backend/wallet/address"
	"perun.network/vc-hub-service/service"
	"polycry.pt/poly-go/sync"
)

type PaymentClient struct {
	balanceMutex sync.Mutex
	Name         string
	balance      *big.Int
	sudtBalance  *big.Int
	Account      *wallet.Account
	wAddr        wire.Address
	Network      types.Network
	PerunClient  *client.Client
	net          *p2p.Net
	channels     chan *PaymentChannel
	AddrResolver service.AddressResolver
	rpcClient    rpc.Client
}

func NewPaymentClient(
	name string,
	network types.Network,
	deployment backend.Deployment,
	rpcUrl string,
	account *wallet.Account,
	key secp256k1.PrivateKey,
	wallet *wallet.EphemeralWallet,
) (*PaymentClient, error) {
	wireAcc := p2p.NewRandomAccount(rand.New(rand.NewSource(time.Now().UnixNano())))
	wireNet, err := p2p.NewP2PBus(wireAcc)
	if err != nil {
		return nil, err
	}
	go wireNet.Bus.Listen(wireNet.Listener)
	addrResolver := service.NewRelayServerResolver(wireAcc)
	addrResolver.SetWire(address.AsParticipant(account.Address()), wireAcc.Address())

	backendRPCClient, err := rpc.Dial(rpcUrl)
	if err != nil {
		return nil, err
	}
	signer := backend.NewSignerInstance(address.AsParticipant(account.Address()).ToCKBAddress(network), key, network)

	ckbClient, err := ckbclient.NewClient(backendRPCClient, *signer, deployment)
	if err != nil {
		return nil, err
	}
	f := funder.NewDefaultFunder(ckbClient, deployment)
	a := adjudicator.NewAdjudicator(ckbClient)
	watcher, err := local.NewWatcher(a)
	if err != nil {
		return nil, err
	}

	wAddr := wireAcc.Address()
	perunClient, err := client.New(wAddr, wireNet.Bus, f, a, wallet, watcher)
	if err != nil {
		return nil, err
	}

	balanceRPC, err := rpc.Dial(rpcUrl)
	if err != nil {
		return nil, err
	}
	p := &PaymentClient{
		Name:         name,
		balance:      big.NewInt(0),
		sudtBalance:  big.NewInt(0),
		Account:      account,
		wAddr:        wAddr,
		Network:      network,
		PerunClient:  perunClient,
		channels:     make(chan *PaymentChannel, 1),
		AddrResolver: addrResolver,
		rpcClient:    balanceRPC,
		net:          wireNet,
	}

	go perunClient.Handle(p, p)
	return p, nil
}

// WalletAddress returns the wallet address of the client.
func (p *PaymentClient) WalletAddress() gpwallet.Address {
	return p.Account.Address()
}

func (p *PaymentClient) WireAddress() wire.Address {
	return p.wAddr
}

func (p *PaymentClient) PeerID() string {
	walletAddr := p.wAddr.(*p2p.Address)
	return walletAddr.ID.String()
}

func (p *PaymentClient) GetSudtBalance() *big.Int {
	p.balanceMutex.Lock()
	defer p.balanceMutex.Unlock()
	return new(big.Int).Set(p.sudtBalance)
}

// GetBalances retrieves the current balances of the client.
func (p *PaymentClient) GetBalances() string {
	p.PollBalances()
	return FormatBalance(p.balance, p.sudtBalance)
}

// OpenChannel opens a new channel with the specified peer and funding.
func (p *PaymentClient) OpenChannel(ctx context.Context, peer gpwallet.Address, amounts map[gpchannel.Asset]float64) *PaymentChannel {
	// We define the channel participants. The proposer always has index 0.
	peerWireAddr, err := p.SetPeerWireAddr(peer)
	if err != nil {
		panic(err)
	}
	participants := []wire.Address{p.WireAddress(), peerWireAddr}

	assets := make([]gpchannel.Asset, len(amounts))
	i := 0
	for a := range amounts {
		assets[i] = a
		i++
	}

	// We create an initial allocation which defines the starting balances.
	initAlloc := gpchannel.NewAllocation(2, assets...)
	for a, amount := range amounts {
		switch a := a.(type) {
		case *asset.Asset:
			if a.IsCKBytes {
				initAlloc.SetAssetBalances(a, []gpchannel.Bal{
					service.CKByteToShannon(big.NewFloat(amount)), // Our initial balance.
					service.CKByteToShannon(big.NewFloat(amount)), // Peer's initial balance.
				})
			} else {
				intAmount := new(big.Int).SetUint64(uint64(amount))
				initAlloc.SetAssetBalances(a, []gpchannel.Bal{
					intAmount, // Our initial balance.
					intAmount, // Peer's initial balance.
				})
			}
		default:
			panic("Asset is not of type *asset.Asset")
		}

	}

	// Prepare the channel proposal by defining the channel parameters.
	challengeDuration := uint64(50) // On-chain challenge duration in seconds.
	proposal, err := client.NewLedgerChannelProposal(
		challengeDuration,
		p.Account.Address(),
		initAlloc,
		participants,
	)
	if err != nil {
		panic(err)
	}

	// Send the proposal.
	ch, err := p.PerunClient.ProposeChannel(ctx, proposal)
	if err != nil {
		panic(err)
	}

	// Start the on-chain event watcher. It automatically handles disputes.
	p.startWatching(ch)

	return newPaymentChannel(ch, assets)
}

func (p *PaymentClient) SetPeerWireAddr(peer gpwallet.Address) (wire.Address, error) {
	peerWireAddr, err := p.AddrResolver.GetWireAddress(peer)
	if err != nil {
		return nil, err
	}
	peerLibp2pAddr, ok := peerWireAddr.(*p2p.Address)
	if !ok {
		panic("peer address is not of type *p2p.Address")
	}
	p.net.Dialer.Register(peerWireAddr, peerLibp2pAddr.String())
	return peerWireAddr, nil
}

func (p *PaymentClient) OpenVirtualChannel(ctx context.Context, vcp client.ChannelProposal, peer gpwallet.Address) *PaymentChannel {
	_, err := p.SetPeerWireAddr(peer)
	if err != nil {
		panic(err)
	}

	ch, err := p.PerunClient.ProposeChannel(ctx, vcp)
	if err != nil {
		panic(err)
	}

	// Start the on-chain event watcher. It automatically handles disputes.
	p.startWatching(ch)

	return newPaymentChannel(ch, vcp.Base().InitBals.Assets)
}

// startWatching starts the dispute watcher for the specified channel.
func (p *PaymentClient) startWatching(ch *client.Channel) {
	go func() {
		err := ch.Watch(p)
		if err != nil {
			fmt.Printf("Watcher returned with error: %v", err)
		}
	}()
}

func (p *PaymentClient) AcceptedChannel() *PaymentChannel {
	return <-p.channels
}

func (p *PaymentClient) Shutdown() {
	p.PerunClient.Close()
	err := p.net.Bus.Close()
	if err != nil {
		fmt.Println("Error closing bus:", err)
	}

}
