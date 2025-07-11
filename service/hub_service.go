package service

import (
	"context"
	"log"
	"math/big"

	gpchannel "perun.network/go-perun/channel"
	basset "perun.network/perun-ckb-backend/channel/asset"
	"perun.network/perun-ckb-backend/wallet/address"
	"perun.network/vc-hub-service/rpc/proto"
)

type HubService struct {
	proto.UnimplementedVCHubServiceServer //always embed for gRPC service impl.
	user                                  *User
	participants                          []address.Participant
	addr                                  address.Participant
}

func (s *HubService) GetAssetsByHub(ctx context.Context, req *proto.GetAssetsByHubRequest) (*proto.GetAssetsByHubResponse, error) {
	// panic("GetAssetsByHub not implemented")
	assets := s.user.GetSupportedAssets()
	protoAssets := make([]*proto.Asset, 0, len(assets))
	for _, a := range assets {
		asset, err := a.MarshalBinary()
		if err != nil {
			log.Println("unable to marshal asset:", err)
			continue
		}
		protoAsset := &proto.Asset{
			Asset: asset,
		}
		protoAssets = append(protoAssets, protoAsset)
	}

	return &proto.GetAssetsByHubResponse{
		Assets: protoAssets,
	}, nil
}

func (s *HubService) GetFees(ctx context.Context, req *proto.GetFeesRequest) (*proto.GetFeesResponse, error) {
	// panic("GetFees not implemented")
	assetsToFunds := make(map[gpchannel.Asset][]*big.Int)

	for _, af := range req.AssetsToFund {
		asset := new(basset.Asset)
		if err := asset.UnmarshalBinary(af.Asset.Asset); err != nil {
			log.Println("unable to unmarshal asset:", err)
			continue
		}
		funds, err := BalanceDistributionToBigFloats(af.BalanceDistribution)
		if err != nil {
			log.Println("unable to convert balance distribution to big.Ints:", err)
			continue
		}
		fundsInBigInt := make([]*big.Int, len(funds))
		for i, f := range funds {
			fundsInBigInt[i] = CKByteToShannon(f)
		}
		assetsToFunds[asset] = fundsInBigInt
	}

	// Call user to get fees
	feeMap, err := s.user.GetFees(assetsToFunds)
	if err != nil {
		log.Println("unable to get fees from user:", err)
		return &proto.GetFeesResponse{}, err
	}
	protoAssetFees := make([]*proto.AssetFee, 0, len(feeMap))
	for asset, feeString := range feeMap {
		gpasset, err := asset.MarshalBinary()
		if err != nil {
			log.Println("unable to marshal asset:", err)
			continue
		}
		assetFee := proto.AssetFee{
			Asset: &proto.Asset{
				Asset: gpasset,
			},
			Fee: feeString,
		}
		protoAssetFees = append(protoAssetFees, &assetFee)
	}
	return &proto.GetFeesResponse{
		AssetFees: protoAssetFees,
	}, nil
}

// TODO: When a participant has a ledger channel with hub, then add it to the participants list.
func (s *HubService) IsParticipantInNetwork(ctx context.Context, req *proto.IsParticipantInNetworkRequest) (*proto.IsParticipantInNetworkResponse, error) {
	addrString := req.Address
	for _, p := range s.participants {
		if p.String() == addrString {
			return &proto.IsParticipantInNetworkResponse{
				IsInNetwork: true,
			}, nil
		}
	}

	return &proto.IsParticipantInNetworkResponse{
		IsInNetwork: false,
	}, nil
}

func (s *HubService) GetPaymentAddress(ctx context.Context, req *proto.GetPaymentAddrRequest) (*proto.GetPaymentAddrResponse, error) {
	paymentAddr := s.addr.String()
	return &proto.GetPaymentAddrResponse{
		PaymentAddress: paymentAddr,
	}, nil
}
