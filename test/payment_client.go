package test

import (
	"math/big"

	"github.com/nervosnetwork/ckb-sdk-go/v2/address"
)

type PaymentClient interface {
	Pay(amount *big.Int, sendingAddr address.Address)
}
