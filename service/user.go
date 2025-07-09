package service

import (
	"math/big"

	gpchannel "perun.network/go-perun/channel"
)

// User represent the owner of this hub-service
// The owner can set the assets supported by this service
// The owner can set a Fee structure for the service
type User struct {
	supportedAssets []gpchannel.Asset // List of assets supported by this service
	feeStructure    FeeStructure      // Fee structure for the service
}

func (u *User) GetSupportedAssets() []gpchannel.Asset {
	return u.supportedAssets
}

func (u *User) GetFees(assetsToFunds map[gpchannel.Asset][]*big.Int) (map[gpchannel.Asset]float64, error) {
	panic("GetFees in user not implemented")
}

type FeeStructure interface {
	//Calculates and returns the fee for the given asset and funds
	GetFee(asset gpchannel.Asset, funds []*big.Int) (*big.Int, error)
}

type SimpleFeeStructure struct {
	feeMap map[gpchannel.Asset]uint32
}
