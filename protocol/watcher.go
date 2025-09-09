package protocol

import (
	"math/big"

	// basset "perun.network/perun-ckb-backend/channel/asset"
	gpchannel "perun.network/go-perun/channel"
)

type Watcher interface {
	FeesPaidForAsset(asset gpchannel.Asset, funds []*big.Int) bool
}

//TODO: Implement a locat watcher. This impl. can include the same FeeStructure given to the user
// upon a call to `FeesPaidForAsset`, watches can get the fees for the given asset from the FeeStructure
// and check on-chain if fees have been paid
