package service

import (
	"context"
	"log"

	"perun.network/vc-hub-service/rpc/proto"
)

type HubService struct {
	proto.UnimplementedVCHubServiceServer //always embed for gRPC service impl.
	user                                  *User
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
