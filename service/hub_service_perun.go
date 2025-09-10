package service

import (
	"context"
	"log"

	"perun.network/vc-hub-service/rpc/proto"
)

func (s *HubService) UpdateChannel(ctx context.Context, req *proto.ChannelUpdateRequest) (*proto.ChannelUpdateResponse, error) {
	//Hub owner can propose updates to the ledger channel in case of recurring fees
	//TODO: Implement this when recurring fee model is implemented
	log.Println("Proposing Channel updates in not supported yet")
	return &proto.ChannelUpdateResponse{
		Msg: &proto.ChannelUpdateResponse_Rejected{
			Rejected: &proto.Rejected{Reason: "Proposing Channel updates in not supported yet"},
		},
	}, nil
}

func (s *HubService) CloseChannel(ctx context.Context, req *proto.CloseChannelRequest) (*proto.ChannelCloseResponse, error) {
	// panic("CloseChannel not implemented")
	cid, user, err := s.GetChannelInfoFromRequest(req.GetChannelId())
	if err != nil {
		return nil, err
	}
	err = user.CloseChannel(ctx, cid)
	if err != nil {
		return &proto.ChannelCloseResponse{Msg: &proto.ChannelCloseResponse_Rejected{Rejected: &proto.Rejected{Reason: err.Error()}}}, err
	}
	return &proto.ChannelCloseResponse{Msg: &proto.ChannelCloseResponse_Close{Close: &proto.SuccessfulClose{ChannelId: cid[:]}}}, nil
}

func (s *HubService) GetChannels(ctx context.Context, req *proto.GetChannelsRequest) (*proto.GetChannelsResponse, error) {
	panic("GetChannels not implemented")
}
