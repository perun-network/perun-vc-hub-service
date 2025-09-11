package service

import (
	"context"
	"fmt"
	"log"

	"perun.network/go-perun/wire/protobuf"
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

func (s *HubService) CloseChannel(ctx context.Context, req *proto.ChannelCloseRequest) (*proto.ChannelCloseResponse, error) {
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
	// panic("GetChannels not implemented")
	u, err := s.getUserFromGetChannelsRequest(req)
	if err != nil {
		return nil, err
	}
	states, actorIndexes := u.GetChannels()
	if len(states) == 0 {
		return &proto.GetChannelsResponse{Msg: &proto.GetChannelsResponse_Rejected{Rejected: &proto.Rejected{Reason: "no channels exists for user"}}}, nil
	}
	pStates := make([]*protobuf.State, len(states))
	pActorIndexes := make([]uint32, len(actorIndexes))
	for i, state := range states {
		pState, err := protobuf.FromState(&state)
		if err != nil {
			return nil, fmt.Errorf("error converting state to protobuf: %w", err)
		}
		pStates[i] = pState
		pActorIndexes[i] = uint32(actorIndexes[i])
	}
	channelStates := &proto.ChannelStates{
		States:    pStates,
		ActorIdxs: pActorIndexes,
	}

	return &proto.GetChannelsResponse{Msg: &proto.GetChannelsResponse_ChannelStates{ChannelStates: channelStates}}, nil
}
