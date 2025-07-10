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

func (u *User) GetFees(assetsToFunds map[gpchannel.Asset][]*big.Int) (map[gpchannel.Asset]string, error) {
	// panic("GetFees in user not implemented")
	fees := make(map[gpchannel.Asset]string)
	for asset, funds := range assetsToFunds {
		fee, err := u.feeStructure.GetFee(asset, funds)
		if err != nil {
			return nil, err
		}
		fees[asset] = ShannonToCKByte(fee).Text('f', 8) // Convert fee to CKByte string with 8 decimal places
	}
	return fees, nil
}

type FeeStructure interface {
	//Calculates and returns the fee for the given asset and balance distribution
	GetFee(asset gpchannel.Asset, funds []*big.Int) (*big.Int, error)
}
