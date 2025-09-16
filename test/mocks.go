package test

import (
	"math/big"

	gpchannel "perun.network/go-perun/channel"
)

type MockFeeWatcher struct {
	flag bool
}

func (m *MockFeeWatcher) SetFeesPaid(flag bool) {
	m.flag = flag
}

func (m *MockFeeWatcher) FeesPaidForAsset(asset gpchannel.Asset, funds []*big.Int) bool {
	return m.flag
}

type MockFeeStructure struct {
	flatFee *big.Int
}

func (m *MockFeeStructure) SetFlatFee(fee *big.Int) {
	m.flatFee = fee
}

func (m *MockFeeStructure) GetFee(asset gpchannel.Asset, funds []*big.Int) (*big.Int, error) {
	return m.flatFee, nil
}
