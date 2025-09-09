package protocol

import (
	"math/big"

	gpchannel "perun.network/go-perun/channel"
)

type FeeStructure interface {
	//Calculates and returns the fee for the given asset and balance distribution
	GetFee(asset gpchannel.Asset, funds []*big.Int) (*big.Int, error)
}
