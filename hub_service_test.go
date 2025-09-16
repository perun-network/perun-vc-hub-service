package main

import (
	"context"
	"log"
	"math/big"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	gpchannel "perun.network/go-perun/channel"

	basset "perun.network/perun-ckb-backend/channel/asset"

	chtest "perun.network/channel-service/test"
	"perun.network/vc-hub-service/rpc/proto"
	"perun.network/vc-hub-service/test"
)

func TestGetAssetsByHub(t *testing.T) {
	testConfig := test.DevnetConfig()
	setup := test.NewTestSetup(t, testConfig)

	//setup
	hubService := setup.HubService.HubService
	hubService.SetFeeStructure(setup.HubProtocol.FeeStructure)
	hubService.SetFeeWatcher(setup.HubProtocol.FeeWatcher)
	log.Println("Hub Service fees and watcher set")
	defer setup.HubService.CleanupFunc()
	defer setup.HubWallet.CleanupFunc()
	for _, fn := range setup.ChannelServiceCleanupFuncs {
		defer fn()
	}
	for _, fn := range setup.WscCleanupFuncs {
		defer fn()
	}

	assetsSetByOwner := make([]gpchannel.Asset, 0)
	assetsSetByOwner = append(assetsSetByOwner, &setup.Asset)
	assetsSetByOwner = append(assetsSetByOwner, &setup.SudtAsset)
	hubService.SetSupportedAssets(assetsSetByOwner)

	// A user will have a client to interact with the hub service
	hubClient := setup.HubService.HubClient

	resp, err := hubClient.GetAssetsByHub(context.Background(), &proto.GetAssetsByHubRequest{})
	assert.NoError(t, err)
	supportedAssets := make([]gpchannel.Asset, 0)
	for _, respAsset := range resp.Assets {
		var asset basset.Asset
		err = asset.UnmarshalBinary(respAsset.Asset)
		assert.NoError(t, err)
		supportedAssets = append(supportedAssets, &asset)
	}

	assert.Equal(t, len(assetsSetByOwner), len(supportedAssets))
	for idx, a := range assetsSetByOwner {
		assert.True(t, a.Equal(supportedAssets[idx]))
	}
}

func TestGetPaymentAddress(t *testing.T) {
	testConfig := test.DevnetConfig()
	setup := test.NewTestSetup(t, testConfig)

	//setup
	hubService := setup.HubService.HubService
	hubService.SetFeeStructure(setup.HubProtocol.FeeStructure)
	hubService.SetFeeWatcher(setup.HubProtocol.FeeWatcher)
	log.Println("Hub Service fees and watcher set")
	defer setup.HubService.CleanupFunc()
	defer setup.HubWallet.CleanupFunc()
	for _, fn := range setup.ChannelServiceCleanupFuncs {
		defer fn()
	}
	for _, fn := range setup.WscCleanupFuncs {
		defer fn()
	}

	// A user will have a client to interact with the hub service
	hubClient := setup.HubService.HubClient

	resp, err := hubClient.GetPaymentAddress(context.Background(), &proto.GetPaymentAddrRequest{})
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.NotEmpty(t, resp.PaymentAddress)

	expectedAddress, err := setup.Participants[2].ToCKBAddress(testConfig.CkbNetworkType).Encode()
	assert.NoError(t, err)
	log.Println("Expected address:", expectedAddress)
	log.Println("Received address:", resp.PaymentAddress)
	assert.Equal(t, expectedAddress, resp.PaymentAddress)
}

func TestIsParticipantInNetwork(t *testing.T) {
	testConfig := test.DevnetConfig()
	setup := test.NewTestSetup(t, testConfig)

	hubService := setup.HubService.HubService
	hubClient := setup.HubService.HubClient
	hubService.SetFeeStructure(setup.HubProtocol.FeeStructure)
	hubService.SetFeeWatcher(setup.HubProtocol.FeeWatcher)
	log.Println("Hub Service fees and watcher set")
	defer setup.HubService.CleanupFunc()
	defer setup.HubWallet.CleanupFunc()
	for _, fn := range setup.ChannelServiceCleanupFuncs {
		defer fn()
	}
	for _, fn := range setup.WscCleanupFuncs {
		defer fn()
	}

	alice := setup.Participants[0]
	aliceCkbAddr, err := alice.ToCKBAddress(testConfig.CkbNetworkType).Encode()
	assert.NoError(t, err)
	bob := setup.Participants[1]
	bobCkbAddr, err := bob.ToCKBAddress(testConfig.CkbNetworkType).Encode()
	assert.NoError(t, err)

	aliceWalletService := setup.WalletServices[0]
	aliceWalletService.SetOpenChannelResponse(true)
	aliceWalletService.SetSignMessageResponse(true)
	aliceWalletService.SetSignTransactionResponse(true)

	hubWalletService := setup.HubWallet.WalletService
	hubWalletService.SetOpenChannelResponse(true)
	hubWalletService.SetSignMessageResponse(true)
	hubWalletService.SetSignTransactionResponse(true)
	hubWalletService.SetSignTransactionResponse(true)

	aliceCSClient := setup.ChannelServiceClients[0]
	ckbAsset := setup.Asset
	assetsmap := map[gpchannel.Asset]float64{
		&ckbAsset: 100.0,
	}
	// Alice opens channel Open channel.
	aliceChannelOpenRequest, err := chtest.NewChannelOpenRequest(setup.Participants[0], setup.Participants[2], assetsmap)
	require.NoError(t, err)

	openChannelResp, err := aliceCSClient.OpenChannel(context.Background(), &aliceChannelOpenRequest)
	log.Println("Channel Opened")
	require.NoError(t, err)
	require.NotNil(t, openChannelResp)

	resp, err := hubClient.IsParticipantInNetwork(context.Background(), &proto.IsParticipantInNetworkRequest{
		Address: aliceCkbAddr,
	})
	require.NoError(t, err)
	log.Println("IsParticipantinNetwork response:", resp.IsInNetwork)
	assert.True(t, resp.IsInNetwork)

	resp, err = hubClient.IsParticipantInNetwork(context.Background(), &proto.IsParticipantInNetworkRequest{
		Address: bobCkbAddr,
	})
	require.NoError(t, err)
	log.Println("IsParticipantinNetwork response:", resp.IsInNetwork)
	assert.False(t, resp.IsInNetwork)
}

func TestGetFees(t *testing.T) {
	testConfig := test.DevnetConfig()
	setup := test.NewTestSetup(t, testConfig)

	//setup
	hubService := setup.HubService.HubService
	hubClient := setup.HubService.HubClient
	defer setup.HubService.CleanupFunc()
	defer setup.HubWallet.CleanupFunc()
	for _, fn := range setup.ChannelServiceCleanupFuncs {
		defer fn()
	}
	for _, fn := range setup.WscCleanupFuncs {
		defer fn()
	}

	// Hub Owner sets fee structure, watcher and assets he/she supports
	supportedAssets := make([]gpchannel.Asset, 0)
	supportedAssets = append(supportedAssets, &setup.Asset)
	supportedAssets = append(supportedAssets, &setup.SudtAsset)
	hubService.SetSupportedAssets(supportedAssets)

	feeStructure := test.MockFeeStructure{}
	fee := int64(100_000_000) // fee is 1 ckbyte (in shannons)
	feeStructure.SetFlatFee(big.NewInt(fee))
	hubService.SetFeeStructure(&feeStructure)

	feeWatcher := &test.MockFeeWatcher{}
	hubService.SetFeeWatcher(feeWatcher)

	//Alice wants to know fees for opening a channel with the hub
	ckbAssetBinary, err := setup.Asset.MarshalBinary()
	assert.NoError(t, err)
	sudtAssetBinary, err := setup.SudtAsset.MarshalBinary()
	assert.NoError(t, err)
	fundingAssets := make([]*proto.AssetAmount, 0)
	fundingAssets = append(fundingAssets, &proto.AssetAmount{
		Asset:               &proto.Asset{Asset: ckbAssetBinary},
		BalanceDistribution: []string{"100", "100"}, // balance distribution of 100 ckbytes each
	})
	fundingAssets = append(fundingAssets, &proto.AssetAmount{
		Asset:               &proto.Asset{Asset: sudtAssetBinary},
		BalanceDistribution: []string{"1000", "1000"}, // balance distribution of 1000 sudt each
	})

	getFeesReq := proto.GetFeesRequest{
		AssetsToFund: fundingAssets,
	}
	resp, err := hubClient.GetFees(context.Background(), &getFeesReq)
	assert.NoError(t, err)

	require.Equal(t, len(supportedAssets), len(resp.AssetFees))
	require.Equal(t, resp.AssetFees[0].Fee, "1.00000000") // 1 ckbyte fee for ckb asset
	require.Equal(t, resp.AssetFees[1].Fee, "1.00000000") // 1 ckbyte fee for sudt asset
}
