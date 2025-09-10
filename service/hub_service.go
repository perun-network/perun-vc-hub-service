package service

import (
	"context"
	"fmt"
	"log"
	"math/big"

	address2 "github.com/nervosnetwork/ckb-sdk-go/v2/address"
	ckbrpc "github.com/nervosnetwork/ckb-sdk-go/v2/rpc"
	"github.com/nervosnetwork/ckb-sdk-go/v2/types"
	"github.com/perun-network/perun-libp2p-wire/p2p"
	"perun.network/go-perun/channel"
	gpchannel "perun.network/go-perun/channel"
	"perun.network/go-perun/channel/persistence"
	gpwallet "perun.network/go-perun/wallet"
	"perun.network/go-perun/watcher/local"
	"perun.network/go-perun/wire"
	"perun.network/perun-ckb-backend/backend"
	"perun.network/perun-ckb-backend/channel/adjudicator"
	basset "perun.network/perun-ckb-backend/channel/asset"
	"perun.network/perun-ckb-backend/channel/funder"
	"perun.network/perun-ckb-backend/client"
	"perun.network/perun-ckb-backend/wallet/address"
	"perun.network/vc-hub-service/rpc/proto"
	"perun.network/vc-hub-service/wallet"
)

type HubService struct {
	proto.UnimplementedVCHubServiceServer //always embed for gRPC service impl.
	user                                  *User
	participants                          []address.Participant
	addr                                  address.Participant
	wsc                                   proto.WalletServiceClient
	net                                   *p2p.Net
	network                               types.Network
	node                                  ckbrpc.Client
	deployment                            backend.Deployment
	wallet                                gpwallet.Wallet
	wireAddr                              wire.Address
	resolver                              AddressResolver
	pr                                    persistence.PersistRestorer
}

// InitializeUser initializes a user with the given participant.
func (s *HubService) InitializeUser(participant address.Participant, wsc proto.WalletServiceClient, w gpwallet.Wallet) (*User, error) {
	log.Printf("Initializing user %s", participant)

	wAddr, err := s.SetWireAddress(participant)
	if err != nil {
		return nil, err
	}
	rs := wallet.NewRemoteSigner(wsc, s.ToCKBAddress(participant), &participant)
	ckbClient, err := client.NewClient(s.node, rs, s.deployment)
	if err != nil {
		return nil, err
	}
	f := funder.NewDefaultFunder(ckbClient, s.deployment)
	adj := adjudicator.NewAdjudicator(ckbClient)
	watcher, err := local.NewWatcher(adj)
	if err != nil {
		return nil, err
	}
	usr, err := NewUser(participant, wAddr, s.net.Bus, f, adj, w, watcher, wsc, s.pr)
	if err != nil {
		return nil, err
	}
	s.user = usr
	return usr, nil
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

// SetWireAddress sets the wire address for the given participant.
func (s HubService) SetWireAddress(participant address.Participant) (wire.Address, error) {
	return s.wireAddr, s.resolver.SetWire(&participant, s.wireAddr)
}

// ToCKBAddress converts a participant address to a CKB address.
func (s HubService) ToCKBAddress(addr address.Participant) address2.Address {
	return addr.ToCKBAddress(s.network)
}

// GetChannelInfoFromRequest returns the channel ID and user from the request.
func (s HubService) GetChannelInfoFromRequest(reqChannelId []byte) (channel.ID, *User, error) {
	cid, err := AsChannelID(reqChannelId)
	if err != nil {
		return channel.ID{}, nil, err
	}
	if s.user == nil {
		return channel.ID{}, nil, fmt.Errorf("user not found")
	}
	return cid, s.user, err
}

func (s HubService) getUserFromGetChannelsRequest(request *proto.GetChannelsRequest) (*User, error) {
	r := request.GetRequester()
	if r == nil {
		return nil, fmt.Errorf("missing requester in GetChannelsRequest")
	}
	var addr address.Participant
	err := addr.UnmarshalBinary(r)
	if err != nil {
		return nil, err
	}

	if s.user != nil {
		if s.user.Participant.Equal(&addr) {
			return s.user, nil
		}
	}

	return nil, fmt.Errorf("user %s not found", addr)
}
