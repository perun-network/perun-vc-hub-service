package test

import (
	"math/big"

	gpchannel "perun.network/go-perun/channel"
)

type MockFeeWatcher struct{}

func (m *MockFeeWatcher) FeesPaidForAsset(asset gpchannel.Asset, funds []*big.Int) bool {
	return true
}

type MockFeeStructure struct{}

func (m *MockFeeStructure) GetFee(asset gpchannel.Asset, funds []*big.Int) (*big.Int, error) {
	return big.NewInt(0), nil
}
