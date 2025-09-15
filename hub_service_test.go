package main

import (
	"context"
	"log"
	"testing"

	"github.com/stretchr/testify/assert"
	gpchannel "perun.network/go-perun/channel"

	basset "perun.network/perun-ckb-backend/channel/asset"

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
