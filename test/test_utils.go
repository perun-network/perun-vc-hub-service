package test

import (
	"log"
	"math/big"

	gpchannel "perun.network/go-perun/channel"

	basset "perun.network/perun-ckb-backend/channel/asset"
	ckbasset "perun.network/perun-ckb-backend/channel/asset"
	"perun.network/vc-hub-service/service"
	"perun.network/vc-hub-service/test/client"
)

func NewAllocation(amounts map[gpchannel.Asset][]float64) *gpchannel.Allocation {
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
		case *ckbasset.Asset:
			if a.IsCKBytes {
				initAlloc.SetAssetBalances(a, []gpchannel.Bal{
					service.CKByteToShannon(big.NewFloat(amount[0])), // Our initial balance.
					service.CKByteToShannon(big.NewFloat(amount[1])), // Peer's initial balance.
				})
			} else {
				intAmount1 := new(big.Int).SetUint64(uint64(amount[0]))
				intAmount2 := new(big.Int).SetUint64(uint64(amount[1]))
				initAlloc.SetAssetBalances(a, []gpchannel.Bal{
					intAmount1, // Our initial balance.
					intAmount2, // Peer's initial balance.
				})
			}
		default:
			panic("Asset is not of type *asset.Asset")
		}

	}
	return initAlloc
}

func PrintBalances(ch *client.PaymentChannel, asset basset.Asset) {
	chAlloc := ch.State().Allocation

	// Constants for formatting CKBytes
	const ckbyteConversionFactor = 100_000_000 // 1 CKByte = 100,000,000 smallest units

	// Log general information
	log.Println("=== Allocation Balances ===")

	// Get Alice's balance (participant 0)
	aliceBalance := chAlloc.Balance(0, &asset)
	aliceBalanceCKBytes := new(big.Float).Quo(new(big.Float).SetInt(aliceBalance), big.NewFloat(ckbyteConversionFactor))

	// Get Bob's balance (participant 1)
	bobBalance := chAlloc.Balance(1, &asset)
	bobBalanceCKBytes := new(big.Float).Quo(new(big.Float).SetInt(bobBalance), big.NewFloat(ckbyteConversionFactor))

	// Print Alice's balance
	log.Printf("Alice's allocation: %s CKBytes", aliceBalanceCKBytes.Text('f', 2))

	// Print Bob's balance
	log.Printf("Bob's allocation: %s CKBytes", bobBalanceCKBytes.Text('f', 2))

	// Calculate the total balance
	totalBalance := new(big.Int).Add(aliceBalance, bobBalance)
	totalBalanceCKBytes := new(big.Float).Quo(new(big.Float).SetInt(totalBalance), big.NewFloat(ckbyteConversionFactor))

	// Print the total channel balance
	log.Printf("Total channel balance: %s CKBytes", totalBalanceCKBytes.Text('f', 2))

	log.Println("===========================")
}
