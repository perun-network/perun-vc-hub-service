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
	"math/big"

	"perun.network/go-perun/channel"
	"perun.network/go-perun/client"
	"perun.network/vc-hub-service/service"
)

type PaymentChannel struct {
	ch     *client.Channel
	assets []channel.Asset
}

// newPaymentChannel creates a new payment channel.
func newPaymentChannel(ch *client.Channel, assets []channel.Asset) *PaymentChannel {
	return &PaymentChannel{
		ch:     ch,
		assets: assets,
	}
}

func (c PaymentChannel) State() *channel.State {
	return c.ch.State().Clone()
}

func (c PaymentChannel) SendPayment(ctx context.Context, amounts map[channel.Asset]float64) {
	// Transfer the given amount from us to peer.
	// Use UpdateBy to update the channel state.
	err := c.ch.Update(ctx, func(state *channel.State) {
		actor := c.ch.Idx()
		peer := 1 - actor
		for a, amount := range amounts {

			if amount < 0 {
				continue
			}

			shannonAmount := service.CKByteToShannon(big.NewFloat(amount))
			state.Allocation.TransferBalance(actor, peer, a, shannonAmount)

		}

	})
	if err != nil {
		panic(err)
	}

}

func (c PaymentChannel) Finalize(ctx context.Context) error {
	// Finalize the channel to enable fast settlement.
	log.Println(" called finalize for channel ", c.ch.ID())
	if !c.ch.State().IsFinal {
		err := c.ch.Update(context.Background(), func(state *channel.State) {
			state.IsFinal = true
		})
		if err != nil {
			return fmt.Errorf("error finalizing channel: %w", err)
		}
	}
	log.Println(" finalized channel ", c.ch.ID())
	return nil
}

// Settle settles the payment channel and withdraws the funds.
func (c PaymentChannel) Settle(ctx context.Context, name string) error {
	// Finalize the channel to enable fast settlement.
	log.Println(name, " called settle for channel ", c.ch.ID())
	if !c.ch.State().IsFinal {
		err := c.ch.Update(context.Background(), func(state *channel.State) {
			state.IsFinal = true
		})
		if err != nil {
			panic(err)
		}
	}
	log.Println(name, " finalized channel ", c.ch.ID())
	// Settle concludes the channel and withdraws the funds.
	err := c.ch.Settle(context.Background(), false)
	if err != nil {
		return fmt.Errorf("error settling channel: %w", err)
	}
	log.Println(name, " settled channel ", c.ch.ID())
	// Close frees up channel resources.
	err = c.ch.Close()
	if err != nil {
		return fmt.Errorf("error closing channel: %w", err)
	}
	log.Println(name, " closed channel ", c.ch.ID())
	return nil
}

func (c PaymentChannel) GetPerunChannel() *client.Channel {
	return c.ch
}
