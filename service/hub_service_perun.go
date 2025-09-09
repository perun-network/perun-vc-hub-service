package service

import (
	"context"

	"perun.network/vc-hub-service/rpc/proto"
)

func (s *HubService) UpdateChannel(ctx context.Context, req *proto.ChannelUpdateRequest) (*proto.ChannelUpdateResponse, error) {
	panic("UpdateChannel not implemented")
}

func (s *HubService) CloseChannel(ctx context.Context, req *proto.CloseChannelRequest) (*proto.ChannelCloseResponse, error) {
	panic("CloseChannel not implemented")
}

func (s *HubService) GetChannels(ctx context.Context, req *proto.GetChannelsRequest) (*proto.GetChannelsResponse, error) {
	panic("GetChannels not implemented")
}
