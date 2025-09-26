package service

import (
	"fmt"
	"math/big"

	"perun.network/go-perun/channel"
	"perun.network/go-perun/client"
	"perun.network/go-perun/wire/protobuf"
	"perun.network/perun-ckb-backend/channel/asset"
	"perun.network/perun-ckb-backend/wallet/address"
	"perun.network/vc-hub-service/rpc/proto"
)

// CKByteToShannon converts a given amount in CKByte to Shannon.
func CKByteToShannon(ckbyteAmount *big.Float) (shannonAmount *big.Int) {
	shannonPerCKByte := new(big.Int).Exp(big.NewInt(10), big.NewInt(8), nil)
	shannonPerCKByteFloat := new(big.Float).SetInt(shannonPerCKByte)
	shannonAmountFloat := new(big.Float).Mul(ckbyteAmount, shannonPerCKByteFloat)
	shannonAmount, _ = shannonAmountFloat.Int(nil)
	return shannonAmount
}

// ShannonToCKByte converts a given amount in Shannon to CKByte.
func ShannonToCKByte(shannonAmount *big.Int) *big.Float {
	shannonPerCKByte := new(big.Int).Exp(big.NewInt(10), big.NewInt(8), nil)
	shannonPerCKByteFloat := new(big.Float).SetInt(shannonPerCKByte)
	shannonAmountFloat := new(big.Float).SetInt(shannonAmount)
	return new(big.Float).Quo(shannonAmountFloat, shannonPerCKByteFloat)
}

// BalanceDistributionToBigFloats converts a slice of string amounts to []*big.Float
func BalanceDistributionToBigFloats(balanceDistribution []string) ([]*big.Float, error) {
	if len(balanceDistribution) == 0 {
		return []*big.Float{}, nil
	}

	result := make([]*big.Float, len(balanceDistribution))

	for i, amountStr := range balanceDistribution {
		if amountStr == "" {
			return nil, fmt.Errorf("empty string at index %d", i)
		}

		amount := new(big.Float)
		_, ok := amount.SetString(amountStr)
		if !ok {
			return nil, fmt.Errorf("invalid number format at index %d: %s", i, amountStr)
		}

		// Check for negative values
		if amount.Sign() < 0 {
			return nil, fmt.Errorf("negative amount at index %d: %s", i, amountStr)
		}

		result[i] = amount
	}

	return result, nil
}

func toCKBAllocation(protoAlloc *protobuf.Allocation) (*channel.Allocation, error) {
	alloc := &channel.Allocation{}
	alloc.Assets = make([]channel.Asset, len(protoAlloc.Assets))
	for i := range protoAlloc.Assets {
		// NOTE: We will assume the first asset will always be CKBytes.
		if i == 0 {
			alloc.Assets[i] = &asset.Asset{
				IsCKBytes: true,
				SUDT:      nil,
			}
		} else {
			alloc.Assets[i] = channel.NewAsset()
		}
		err := alloc.Assets[i].UnmarshalBinary(protoAlloc.Assets[i])
		if err != nil {
			return nil, fmt.Errorf("%d'th asset: %w", i, err)
		}
	}
	alloc.Locked = make([]channel.SubAlloc, len(protoAlloc.Locked))
	for i := range protoAlloc.Locked {
		locked, err := protobuf.ToSubAlloc(protoAlloc.Locked[i])
		if err != nil {
			return nil, fmt.Errorf("%d'th sub alloc: %w", i, err)
		}
		alloc.Locked[i] = locked
	}
	alloc.Balances = protobuf.ToBalances(protoAlloc.Balances)

	return alloc, nil
}

// AsChannelID converts a byte slice to a channel ID.
func AsChannelID(in []byte) (channel.ID, error) {
	id := channel.ID{}
	n := copy(id[:], in)
	if n != len(id) {
		return channel.ID{}, fmt.Errorf("channel id too short: expected %d bytes, got %d", len(id), n)
	}
	return id, nil
}

// get assets for a given participant. This participant must be part of one of the channels.
func getAssetsForParticipant(channels map[channel.ID]*client.Channel, part address.Participant) ([]channel.Asset, error) {
	for _, ch := range channels {
		participants := ch.Params().Parts
		for _, p := range participants {
			if p.Equal(&part) {
				return ch.State().Assets, nil
			}
		}
	}
	return nil, fmt.Errorf("participant not found in any channel")
}

// convert channel assets to proto assets
func channelAssetsToProtoAsset(assets []channel.Asset) ([]*proto.Asset, error) {
	protoAssets := make([]*proto.Asset, len(assets))
	for i, a := range assets {
		data, err := a.MarshalBinary()
		if err != nil {
			return nil, fmt.Errorf("marshalling asset %d: %w", i, err)
		}
		protoAssets[i] = &proto.Asset{Asset: data}
	}
	return protoAssets, nil
}
