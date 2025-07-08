package service

import (
	"context"
	"log"

	gpchannel "perun.network/go-perun/channel"
	"perun.network/vc-hub-service/rpc/proto"
)

type HubService struct {
	proto.UnimplementedVCHubServiceServer //always embed for gRPC service impl.
	user                                  *User
	assets                                []gpchannel.Asset // List of assets supported by this service
}

func (s *HubService) GetAssetsByHub(ctx context.Context, req *proto.GetAssetsByHubRequest) (*proto.GetAssetsByHubResponse, error) {
	// panic("GetAssetsByHub not implemented")
	protoAssets := make([]*proto.Asset, 0, len(s.assets))
	for _, a := range s.assets {
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
