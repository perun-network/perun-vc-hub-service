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

// // Convert single protobuf BigInt to *big.Int
// func ProtoBigIntToBigInt(pb *pb.BigInt) *big.Int {
// 	if pb == nil || len(pb.Data) == 0 {
// 		return big.NewInt(0)
// 	}
// 	return new(big.Int).SetBytes(pb.Data)
// }

// // Convert *big.Int to protobuf BigInt
// func BigIntToProtoBigInt(bi *big.Int) *pb.BigInt {
// 	if bi == nil {
// 		return &pb.BigInt{Data: []byte{}}
// 	}
// 	return &pb.BigInt{Data: bi.Bytes()}
// }

// // convert a slice of protobuf BigInts to a slice of *big.Int
// func BalanceDistributionToBigInts(balanceDistribution []*pb.BigInt) []*big.Int {
// 	if balanceDistribution == nil {
// 		return nil
// 	}

// 	result := make([]*big.Int, len(balanceDistribution))
// 	for i, protoBigInt := range balanceDistribution {
// 		result[i] = ProtoBigIntToBigInt(protoBigInt)
// 	}
// 	return result
// }
