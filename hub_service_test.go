package main

import (
	"context"
	"fmt"
	"log"
	"math/big"
	"testing"
	"time"

	"polycry.pt/poly-go/sync"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	gpchannel "perun.network/go-perun/channel"
	gpclient "perun.network/go-perun/client"
	gpwire "perun.network/go-perun/wire"

	basset "perun.network/perun-ckb-backend/channel/asset"

	"perun.network/vc-hub-service/rpc/proto"
	"perun.network/vc-hub-service/test"
	"perun.network/vc-hub-service/test/client"
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

func TestIsAddressInNetwork(t *testing.T) {
	testConfig := test.DevnetConfig()
	setup := test.NewPaymentClientSetup(t, testConfig)

	hubService := setup.HubService.HubService
	hubClient := setup.HubService.HubClient
	hubService.SetFeeStructure(setup.HubProtocol.FeeStructure)
	feeWatcher := &test.MockFeeWatcher{}
	feeWatcher.SetFeesPaid(true)
	hubService.SetFeeWatcher(feeWatcher)
	log.Println("Hub Service fees and watcher set")
	defer setup.HubService.CleanupFunc()
	defer setup.HubWallet.CleanupFunc()
	for _, fn := range setup.PaymentClientsCleanUp {
		defer fn()
	}

	alice := setup.Participants[0]
	aliceCkbAddr, err := alice.ToCKBAddress(testConfig.CkbNetworkType).Encode()
	assert.NoError(t, err)
	bob := setup.Participants[1]
	bobCkbAddr, err := bob.ToCKBAddress(testConfig.CkbNetworkType).Encode()
	assert.NoError(t, err)
	hubAddr := setup.Participants[2]

	hubWalletService := setup.HubWallet.WalletService
	hubWalletService.SetOpenChannelResponse(true)
	hubWalletService.SetSignMessageResponse(true)
	hubWalletService.SetSignTransactionResponse(true)
	hubWalletService.SetSignTransactionResponse(true)

	alicePC := setup.PaymentClients[0]

	chAlice := alicePC.OpenChannel(setup.Ctx, &hubAddr, map[gpchannel.Asset]float64{
		&setup.Asset: 100.0,
	})
	require.NotNil(t, chAlice)
	log.Println("Alice opened channel with hub with id:", chAlice.State().ID)
	log.Println("Checking whether Alice is in network: ", aliceCkbAddr)
	resp, err := hubClient.IsAddressInNetwork(context.Background(), &proto.IsAddressInNetworkRequest{
		Address: aliceCkbAddr,
	})
	require.NoError(t, err)
	require.IsType(t, &proto.IsAddressInNetworkResponse_Peer{}, resp.Msg)
	respInfo := resp.Msg.(*proto.IsAddressInNetworkResponse_Peer)
	log.Printf("Address %v is present in network with L2 address", respInfo.Peer.WireAddress)

	resp, err = hubClient.IsAddressInNetwork(context.Background(), &proto.IsAddressInNetworkRequest{
		Address: bobCkbAddr,
	})
	require.NoError(t, err)
	require.IsType(t, &proto.IsAddressInNetworkResponse_Rejected{}, resp.Msg)
	chAlice.Settle(setup.Ctx, "Alice")
	log.Println("TestIsParticipantInNetwork finished")
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

func TestHappy(t *testing.T) {
	testConfig := test.DevnetConfig()
	setup := test.NewPaymentClientSetup(t, testConfig)

	//setuptask
	hubService := setup.HubService.HubService
	defer setup.HubService.CleanupFunc()
	defer setup.HubWallet.CleanupFunc()
	for _, fn := range setup.PaymentClientsCleanUp {
		defer fn()
	}

	hubPart := setup.Participants[2]
	hubWalletService := setup.HubWallet.WalletService
	hubWalletService.SetOpenChannelResponse(true)
	hubWalletService.SetUpdateNotificationResponse(true)
	hubWalletService.SetSignMessageResponse(true)
	hubWalletService.SetSignTransactionResponse(true)

	log.Println("Alice balances: ", setup.PaymentClients[0].GetBalances())
	log.Println("Bob balances: ", setup.PaymentClients[1].GetBalances())

	hubService.SetFeeStructure(setup.HubProtocol.FeeStructure)
	feeWatcher := &test.MockFeeWatcher{}
	feeWatcher.SetFeesPaid(true)
	hubService.SetFeeWatcher(feeWatcher)
	alicePC := setup.PaymentClients[0]

	chAlice := alicePC.OpenChannel(setup.Ctx, &hubPart, map[gpchannel.Asset]float64{
		&setup.Asset: 100.0,
	})
	require.NotNil(t, chAlice)
	log.Println("Alice opened channel with hub with id:", chAlice.State().ID)

	//Bob opens a ledger channel with hub
	bobPC := setup.PaymentClients[1]
	chBob := bobPC.OpenChannel(setup.Ctx, &hubPart, map[gpchannel.Asset]float64{
		&setup.Asset: 100.0,
	})
	require.NotNil(t, chBob)
	log.Println("Bob opened channel with hub with id:", chBob.State().ID)

	log.Println(">>>>>>>>>>>>>\n Virtual Channel testing starts >>>>>>>>>>>>> \n>>>>>>>>>>>>>")
	challengeDuration := uint64(30)
	assetVCMap := map[gpchannel.Asset][]float64{
		&setup.Asset: {50.0, 50.0}, // 50 ckbytes each
	}
	initBals := test.NewAllocation(assetVCMap)
	peers := []gpwire.Address{alicePC.WireAddress(), bobPC.WireAddress()}
	parents := []gpchannel.ID{chAlice.State().ID, chBob.State().ID}
	//gpchannel.Index is basically uint16
	// indexMapAlice maps who locks funds in parent channel chAliceHub for the VC participants
	// the VC proposer's funds are locked by the participant indexMapAlice[0] in chAliceHub
	// the VC proposee's funds are locked by the participant indexMapAlice[1] in chAliceHub
	// similarly for indexMapBob
	indexMapAlice := []gpchannel.Index{0, 1}
	indexMapBob := []gpchannel.Index{1, 0}
	indexMaps := [][]gpchannel.Index{indexMapAlice, indexMapBob}
	var aux gpchannel.Aux
	copy(aux[:gpchannel.IDLen], chAlice.State().ID[:])
	copy(aux[gpchannel.IDLen:], chBob.State().ID[:])
	vcp, err := gpclient.NewVirtualChannelProposal(challengeDuration, &setup.Participants[0], initBals, peers, parents, indexMaps, gpclient.WithAux(aux))
	assert.NoError(t, err)
	log.Println("Virtual channel proposal created by Alice")

	// open vc
	chAliceVC := alicePC.OpenVirtualChannel(setup.Ctx, vcp, bobPC.Account.Address())
	chBobVC := bobPC.AcceptedChannel()
	assert.NotNil(t, chAliceVC)
	assert.NotNil(t, chBobVC)
	assert.Equal(t, chAliceVC.State().ID, chBobVC.State().ID)
	log.Println("Alice opened virtual channel with Bob")

	// update vc
	log.Println("Alice updating virtual channel")
	chAliceVC.SendPayment(setup.Ctx, map[gpchannel.Asset]float64{
		&setup.Asset: 20.0,
	})
	chBobVC.SendPayment(setup.Ctx, map[gpchannel.Asset]float64{
		&setup.Asset: 10.0,
	})

	test.PrintBalances(chBobVC, setup.Asset)
	//finalize vc
	log.Println("Bob finalizing virtual channel")
	err = chBobVC.Finalize(setup.Ctx)
	assert.NoError(t, err)
	// close vc
	log.Println("Alice and Bob closing virtual channel")
	vcs := map[string]*client.PaymentChannel{
		"Alice": chAliceVC,
		"Bob":   chBobVC,
	}
	var success sync.WaitGroup
	errs := make(chan error, 10)
	success.Add(len(vcs))
	// create go routines to settle vc and wait for all to finish
	// need a way to log errors back from if a goroutine publishes any error
	for name, vc := range vcs {
		go func(c *client.PaymentChannel) {
			fmt.Println("Settling vc for ", name)
			err = c.Settle(setup.Ctx, name)
			assert.NoError(t, err)
			if err != nil {
				errs <- err
			}
			fmt.Println("Settled vc for ", name)
			success.Done()
		}(vc)
	}

	select {
	case <-success.WaitCh():
		fmt.Println("All VCs settled successfully")
	case err := <-errs:
		fmt.Println("Error settling VCs: ", err)
		t.Fatalf("Error in go-routine: %v", err)
	}
	fmt.Println("Closing Parent Channels")
	chAlice.Settle(setup.Ctx, "Alice")
	fmt.Println("Settled Alice's channel")
	chBob.Settle(setup.Ctx, "Bob")
	fmt.Println("Settled Bob's channel")
	log.Println("Happy2 Test End")
}

func TestDispute(t *testing.T) {
	testConfig := test.DevnetConfig()
	setup := test.NewPaymentClientSetup(t, testConfig)

	//setuptask
	hubService := setup.HubService.HubService
	// hubClient := setup.HubService.HubClient
	defer setup.HubService.CleanupFunc()
	defer setup.HubWallet.CleanupFunc()
	for _, fn := range setup.PaymentClientsCleanUp {
		defer fn()
	}
	hubWalletService := setup.HubWallet.WalletService
	hubWalletService.SetOpenChannelResponse(true)
	hubWalletService.SetUpdateNotificationResponse(true)
	hubWalletService.SetSignMessageResponse(true)
	hubWalletService.SetSignTransactionResponse(true)
	hubPart := setup.Participants[2]

	log.Println("Alice balances: ", setup.PaymentClients[0].GetBalances())
	log.Println("Bob balances: ", setup.PaymentClients[1].GetBalances())

	hubService.SetFeeStructure(setup.HubProtocol.FeeStructure)
	feeWatcher := &test.MockFeeWatcher{}
	feeWatcher.SetFeesPaid(true)
	hubService.SetFeeWatcher(feeWatcher)
	//Alice opens a ledger channel with hub
	alicePC := setup.PaymentClients[0]

	chAlice := alicePC.OpenChannel(setup.Ctx, &hubPart, map[gpchannel.Asset]float64{
		&setup.Asset: 100.0,
	})
	require.NotNil(t, chAlice)
	log.Println("Alice opened channel with hub with id:", chAlice.State().ID)

	//Bob opens a ledger channel with hub
	bobPC := setup.PaymentClients[1]
	chBob := bobPC.OpenChannel(setup.Ctx, &hubPart, map[gpchannel.Asset]float64{
		&setup.Asset: 100.0,
	})
	require.NotNil(t, chBob)
	log.Println("Bob opened channel with hub with id:", chBob.State().ID)

	log.Println(">>>>>>>>>>>>>\n Virtual Channel testing starts >>>>>>>>>>>>> \n>>>>>>>>>>>>>")
	challengeDuration := uint64(30)
	assetVCMap := map[gpchannel.Asset][]float64{
		&setup.Asset: {50.0, 50.0}, // 50 ckbytes each
	}
	initBals := test.NewAllocation(assetVCMap)
	peers := []gpwire.Address{alicePC.WireAddress(), bobPC.WireAddress()}
	parents := []gpchannel.ID{chAlice.State().ID, chBob.State().ID}
	//gpchannel.Index is basically uint16
	// indexMapAlice maps who locks funds in parent channel chAliceHub for the VC participants
	// the VC proposer's funds are locked by the participant indexMapAlice[0] in chAliceHub
	// the VC proposee's funds are locked by the participant indexMapAlice[1] in chAliceHub
	// similarly for indexMapBob
	indexMapAlice := []gpchannel.Index{0, 1}
	indexMapBob := []gpchannel.Index{1, 0}
	indexMaps := [][]gpchannel.Index{indexMapAlice, indexMapBob}
	var aux gpchannel.Aux

	copy(aux[:gpchannel.IDLen], chAlice.State().ID[:]) // Alice at position 0
	copy(aux[gpchannel.IDLen:], chBob.State().ID[:])   // Bob at position IDLen
	log.Println("chAlice ID: ", chAlice.State().ID)
	log.Println("chBob ID: ", chBob.State().ID)
	log.Println("aux is: ", aux)
	vcp, err := gpclient.NewVirtualChannelProposal(challengeDuration, &setup.Participants[0], initBals, peers, parents, indexMaps, gpclient.WithAux(aux))
	assert.NoError(t, err)
	log.Println("Virtual channel proposal created by Alice")
	log.Println("Aux in vcp is: ", vcp.Aux)
	// open vc
	chAliceVC := alicePC.OpenVirtualChannel(setup.Ctx, vcp, bobPC.Account.Address())
	chBobVC := bobPC.AcceptedChannel()
	assert.NotNil(t, chAliceVC)
	assert.NotNil(t, chBobVC)
	assert.Equal(t, chAliceVC.State().ID, chBobVC.State().ID)
	log.Println("Alice opened virtual channel with Bob")

	// update vc
	log.Println("Alice updating virtual channel")
	chAliceVC.SendPayment(setup.Ctx, map[gpchannel.Asset]float64{
		&setup.Asset: 20.0,
	})
	chBobVC.SendPayment(setup.Ctx, map[gpchannel.Asset]float64{
		&setup.Asset: 10.0,
	})

	test.PrintBalances(chBobVC, setup.Asset)
	//Register Disputes
	log.Println("Aux params for Alice are: ", chAliceVC.GetPerunChannel().Params().Aux)
	log.Println("Aux params for Bob are: ", chBobVC.GetPerunChannel().Params().Aux)
	//Alice registers dispute on VC, and thus on lc between her and hub
	//Hub should react to this by registering a dispute on his channel with Bob automatically.
	//Hub is responsible for closing chBob
	// Alice is responsible for closing chAlice
	chHubAlice := setup.HubService.User.Channels[chAlice.State().ID]
	// log.Println("All channels in hub right now", setup.HubService.User.Channels)
	assert.NotNil(t, chHubAlice)
	chHubBob := setup.HubService.User.Channels[chBob.State().ID]
	// log.Println("All channels in hub right now", setup.HubService.User.Channels)
	assert.NotNil(t, chHubBob)

	waitTimeout := time.Second * 3
	chs := []*gpclient.Channel{chAlice.GetPerunChannel(), chBob.GetPerunChannel()}
	perm := []int{1, 0} // Bob first, then Alice
	for _, i := range perm {
		err := gpclient.NewTestChannel(chs[i]).Register(setup.Ctx)
		assert.NoErrorf(t, err, "register channel: %d", i)
		time.Sleep(waitTimeout) // Sleep to ensure that events have been processed and local client states have been updated.
	}
	// err = gpclient.NewTestChannel(chAliceVC.GetPerunChannel()).Register(context.Background())
	// assert.NoError(t, err)
	// time.Sleep(waitTimeout)
	// // log.Println("Bob finalizing virtual channel")
	// err = chBobVC.Finalize(setup.Ctx)
	// assert.NoError(t, err)
	// // close vc
	// log.Println("Alice and Bob closing virtual channel")
	// // vcs := []*client.PaymentChannel{chAliceVC, chBobVC}
	// vcs := map[string]*client.PaymentChannel{
	// 	"Alice": chAliceVC,
	// 	"Bob":   chBobVC,
	// }
	// var success sync.WaitGroup
	// // errs               chan error
	// errs := make(chan error, 10)
	// success.Add(len(vcs))
	// // create go routines to settle vc and wait for all to finish
	// // need a way to log errors back from if a goroutine publishes any error
	// for name, vc := range vcs {
	// 	go func(c *client.PaymentChannel) {
	// 		fmt.Println("Settling vc for ", name)
	// 		err = c.Settle(setup.Ctx, name)
	// 		assert.NoError(t, err)
	// 		if err != nil {
	// 			errs <- err
	// 		}
	// 		fmt.Println("Settled vc for ", name)
	// 		success.Done()
	// 	}(vc)
	// }

	// select {
	// case <-success.WaitCh():
	// 	fmt.Println("All VCs settled successfully")
	// case err := <-errs:
	// 	fmt.Println("Error settling VCs: ", err)
	// 	t.Fatalf("Error in go-routine: %v", err)
	// }
	// fmt.Println("Closing Parent Channels")
	// chAlice.Settle(setup.Ctx, "Alice")
	// fmt.Println("Settled Alice's channel")
	// chBob.Settle(setup.Ctx, "Bob")
	// fmt.Println("Settled Bob's channel")
	// alicePC.Shutdown()
	// bobPC.Shutdown()
	// log.Println("Dispute Test End")
}
