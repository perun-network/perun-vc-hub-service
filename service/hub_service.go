package service

import (
	"context"
	"fmt"
	"log"
	"math/big"
	"math/rand"
	"time"

	address2 "github.com/nervosnetwork/ckb-sdk-go/v2/address"
	ckbrpc "github.com/nervosnetwork/ckb-sdk-go/v2/rpc"
	"github.com/nervosnetwork/ckb-sdk-go/v2/types"
	"github.com/perun-network/perun-libp2p-wire/p2p"

	gpchannel "perun.network/go-perun/channel"
	gpwallet "perun.network/go-perun/wallet"
	"perun.network/go-perun/watcher/local"
	"perun.network/go-perun/wire"

	"perun.network/perun-ckb-backend/backend"
	"perun.network/perun-ckb-backend/channel/adjudicator"
	basset "perun.network/perun-ckb-backend/channel/asset"
	"perun.network/perun-ckb-backend/channel/funder"
	"perun.network/perun-ckb-backend/client"
	"perun.network/perun-ckb-backend/wallet/address"
	"perun.network/perun-ckb-backend/wallet/external"

	"perun.network/vc-hub-service/protocol"
	"perun.network/vc-hub-service/rpc/proto"

	chproto "perun.network/channel-service/rpc/proto"
	chwallet "perun.network/channel-service/wallet"
)

// participantRegistry is a global participant registry for the hub service.
var participantRegistry = NewLocalParticipantRegistry()

type HubService struct {
	proto.UnimplementedVCHubServiceServer //always embed for gRPC service impl.
	user                                  *User
	addr                                  address.Participant
	wsc                                   chproto.WalletServiceClient
	net                                   *p2p.Net
	network                               types.Network
	node                                  ckbrpc.Client
	deployment                            backend.Deployment
	wallet                                gpwallet.Wallet
	wireAddr                              wire.Address
	resolver                              AddressResolver
}

// NewChannelService creates a new ChannelService.
func NewHubService(c chproto.WalletServiceClient, network types.Network, nodeURL string, deployment backend.Deployment, res AddressResolver, addr address.Participant) (*HubService, error) {
	node, err := ckbrpc.Dial(nodeURL)
	if err != nil {
		return nil, err
	}

	wireAcc := p2p.NewRandomAccount(rand.New(rand.NewSource(time.Now().UnixNano())))

	wireNet, err := p2p.NewP2PBus(wireAcc)
	if err != nil {
		return nil, fmt.Errorf("error creating wire net: %w", err)
	}

	go wireNet.Bus.Listen(wireNet.Listener)

	if res == nil {
		res = NewRelayServerResolver(wireAcc)
	}

	hs := &HubService{
		user:       nil,
		addr:       addr,
		wsc:        c,
		net:        wireNet,
		network:    network,
		node:       node,
		deployment: deployment,
		wallet:     external.NewWallet(chwallet.NewExternalClient(c)),
		wireAddr:   wireAcc.Address(),
		resolver:   res,
	}

	return hs, nil
}

// InitializeUser initializes a user with the given participant.
func (s *HubService) InitializeUser(participant address.Participant, wsc chproto.WalletServiceClient, w gpwallet.Wallet) (*User, error) {
	log.Printf("Initializing user %s", participant)

	wAddr, err := s.SetWireAddress(participant)
	if err != nil {
		return nil, err
	}
	rs := chwallet.NewRemoteSigner(wsc, s.ToCKBAddress(participant), &participant)
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
	usr, err := NewUser(participant, wAddr, s.net.Bus, f, adj, w, watcher, wsc, s.network)
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

func (s *HubService) IsAddressInNetwork(ctx context.Context, req *proto.IsAddressInNetworkRequest) (*proto.IsAddressInNetworkResponse, error) {
	addr := req.Address
	res, error := participantRegistry.IsAddressInNetwork(addr)
	if error != nil {
		return nil, fmt.Errorf("error while querying if address is in network: %w", error)
	}

	// If address is not in network, return Rejected message
	if !res {
		return &proto.IsAddressInNetworkResponse{
			Msg: &proto.IsAddressInNetworkResponse_Rejected{
				Rejected: &proto.Rejected{
					Reason: fmt.Sprintf("Address %s not in network", addr),
				},
			},
		}, nil
	}
	part, err := participantRegistry.GetParticipant(addr)
	if err != nil {
		return nil, fmt.Errorf("error while getting participant from registry: %w", err)
	}
	wireAddr, err := s.resolver.GetWireAddress(&part)
	if err != nil {
		return nil, fmt.Errorf("error while getting wire address from resolver: %w", err)
	}
	libp2pAddr, ok := wireAddr.(*p2p.Address)
	if !ok {
		return nil, fmt.Errorf("address returned from resolver is not a libp2p address")
	}
	assets, err := getAssetsForParticipant(s.user.Channels, part)
	if err != nil {
		return nil, fmt.Errorf("cannot get gpchannel assets for participant: %w", err)
	}
	partAssets, err := channelAssetsToProtoAsset(assets)
	if err != nil {
		return nil, fmt.Errorf("cannot marshall gpchannel assets: %w", err)
	}
	return &proto.IsAddressInNetworkResponse{
		Msg: &proto.IsAddressInNetworkResponse_Peer{
			Peer: &proto.PeerInformation{
				WireAddress: libp2pAddr.String(),
				Assets:      partAssets,
			},
		},
	}, nil
}

func (s *HubService) GetPaymentAddress(ctx context.Context, req *proto.GetPaymentAddrRequest) (*proto.GetPaymentAddrResponse, error) {
	ckbAddr, err := s.ToCKBAddress(s.addr).Encode()
	if err != nil {
		return nil, err
	}
	return &proto.GetPaymentAddrResponse{
		PaymentAddress: ckbAddr,
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
func (s HubService) GetChannelInfoFromRequest(reqChannelId []byte) (gpchannel.ID, *User, error) {
	cid, err := AsChannelID(reqChannelId)
	if err != nil {
		return gpchannel.ID{}, nil, err
	}
	if s.user == nil {
		return gpchannel.ID{}, nil, fmt.Errorf("user not found")
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

func (c HubService) Close() error {
	return c.net.Bus.Close()
}

func (s *HubService) SetFeeStructure(fs protocol.FeeStructure) error {
	// TODO: Add authentication so that only the owner can call them
	if s.user == nil {
		return fmt.Errorf("user not initialized")
	}
	s.user.feeStructure = fs
	return nil
}

func (s *HubService) SetFeeWatcher(fw protocol.Watcher) error {
	//TODO: Add authentication so that only the owner can call them
	if s.user == nil {
		return fmt.Errorf("user not initialized")
	}
	s.user.feeWatcher = fw
	return nil
}

func (s *HubService) SetSupportedAssets(assets []gpchannel.Asset) error {
	//TODO: Add authentication so that only the owner can call them
	if s.user == nil {
		return fmt.Errorf("user not initialized")
	}
	s.user.supportedAssets = assets
	return nil
}
