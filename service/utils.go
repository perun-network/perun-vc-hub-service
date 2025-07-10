package service

import (
	"fmt"
	"math/big"
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
