package test

import (
	"context"
	"log"
	"net"
	"testing"
	"time"

	"github.com/decred/dcrd/dcrec/secp256k1/v4"
	"github.com/nervosnetwork/ckb-sdk-go/v2/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
	"perun.network/vc-hub-service/test/client"
	"perun.network/vc-hub-service/test/deployment"

	"perun.network/perun-ckb-backend/backend"
	"perun.network/perun-ckb-backend/channel/asset"
	ckbwallet "perun.network/perun-ckb-backend/wallet"
	ckbaddr "perun.network/perun-ckb-backend/wallet/address"
	"perun.network/perun-ckb-backend/wallet/external"

	chproto "perun.network/channel-service/rpc/proto"
	chtest "perun.network/channel-service/service/test"
	chwallet "perun.network/channel-service/wallet"
)

const (
	testDuration = 60 * time.Second
)

// Setup contains all the necessary information for testing.
type PaymentClientSetup struct {
	t          *testing.T
	Deployment backend.Deployment
	SUDTInfo   deployment.SUDTInfo

	WalletAccs            []*ckbwallet.Account
	AccKeys               []*secp256k1.PrivateKey
	EphWallet             *ckbwallet.EphemeralWallet
	Participants          []ckbaddr.Participant
	PaymentClients        []*client.PaymentClient
	PaymentClientsCleanUp []func()

	HubWallet   HubWalletInfo
	HubService  HubServiceInfo
	HubProtocol HubProtocolInfo

	Asset     asset.Asset
	SudtAsset asset.Asset

	CtxCleanup func()
	Ctx        context.Context
}

// NewTestSetup creates a new setup for testing.
func NewPaymentClientSetup(t *testing.T, testConfig *TestConfig) *PaymentClientSetup {
	setup := &PaymentClientSetup{}
	setup.t = t
	ctx, cancel := context.WithTimeout(context.Background(), testDuration)
	setup.Ctx = ctx
	setup.CtxCleanup = cancel
	sudtOwnerLockArg, err := parseSUDTOwnerLockArg(testConfig.NetworkDirectory + "/" + SUDTOwnerLockArgFile)
	require.NoError(t, err, "error getting SUDT owner lock arg")

	migrationPath := testConfig.NetworkDirectory + "/" + ContractMigrationsPath
	migrationVCPath := testConfig.NetworkDirectory + "/" + ContractMigrationsVCPath
	d, sudtInfo, err := deployment.GetDeployment(migrationPath, migrationVCPath, testConfig.NetworkDirectory+"/"+SystemScriptsDir, sudtOwnerLockArg)
	require.NoError(t, err, "error getting deployment")
	setup.Deployment = d
	setup.SUDTInfo = sudtInfo

	alicePrivateKey, err := deployment.GetKey(testConfig.NetworkDirectory + "/" + AlicePKFile)
	require.NoError(t, err, "error getting alice's private key")

	bobPrivateKey, err := deployment.GetKey(testConfig.NetworkDirectory + "/" + BobPKFile)
	require.NoError(t, err, "error getting bob's private key")

	hubOwnerPrivateKey, err := deployment.GetKey(testConfig.NetworkDirectory + "/" + HubOwnerPKFile)
	require.NoError(t, err, "error getting hub's private key")

	pubKeys := []*secp256k1.PublicKey{alicePrivateKey.PubKey(), bobPrivateKey.PubKey(), hubOwnerPrivateKey.PubKey()}
	parts, err := MakeParticipants(pubKeys)
	setup.Participants = parts
	require.NoError(t, err, "error making participants")

	aliceAccount := ckbwallet.NewAccountFromPrivateKey(alicePrivateKey)
	bobAccount := ckbwallet.NewAccountFromPrivateKey(bobPrivateKey)
	hubAccount := ckbwallet.NewAccountFromPrivateKey(hubOwnerPrivateKey)
	w := ckbwallet.NewEphemeralWallet()
	w.AddAccount(aliceAccount)
	w.AddAccount(bobAccount)
	w.AddAccount(hubAccount)
	setup.EphWallet = w
	setup.WalletAccs = []*ckbwallet.Account{aliceAccount, bobAccount, hubAccount}
	setup.AccKeys = []*secp256k1.PrivateKey{alicePrivateKey, bobPrivateKey, hubOwnerPrivateKey}

	log.Printf("Participants: %v", parts)
	//setup payment clients
	alicePC, alicePCCleanUp, err := setupPaymentClient(t, "alice", testConfig, d, aliceAccount, *alicePrivateKey, w, parts[0])
	require.NoError(t, err, "error creating alice's payment client")
	setup.PaymentClients = append(setup.PaymentClients, alicePC)
	setup.PaymentClientsCleanUp = append(setup.PaymentClientsCleanUp, alicePCCleanUp)
	bobPC, bobPCCleanup, err := setupPaymentClient(t, "bob", testConfig, d, bobAccount, *bobPrivateKey, w, parts[1])
	require.NoError(t, err, "error creating bob's payment client")
	setup.PaymentClients = append(setup.PaymentClients, bobPC)
	setup.PaymentClientsCleanUp = append(setup.PaymentClientsCleanUp, bobPCCleanup)

	//setup hub wallet service and hub service
	hubWSC, hubWS, hubWSCCleanup := setup.setupWalletService(t, "hub", hubAccount, hubOwnerPrivateKey, Network)
	setup.HubWallet = HubWalletInfo{
		CleanupFunc:   hubWSCCleanup,
		WSClient:      hubWSC,
		WalletService: hubWS,
	}
	HubClient, HubService, HubCleanup := setupHubService(t, "hub", hubWSC, Network, testConfig.RPCNodeURL, d, nil, parts[2])
	setup.HubService = HubServiceInfo{
		HubService:  HubService,
		CleanupFunc: HubCleanup,
		HubClient:   HubClient,
	}
	hubUser, err := HubService.InitializeUser(parts[2], hubWSC, external.NewWallet(chwallet.NewExternalClient(hubWSC)))
	require.NoError(t, err, "error initializing hub user")
	setup.HubService.User = hubUser
	log.Println("Initialized hub user")

	setup.Asset = asset.Asset{
		IsCKBytes: true,
		SUDT:      nil,
	}
	setup.SudtAsset = asset.Asset{
		IsCKBytes: false,
		SUDT:      asset.NewSUDT(*sudtInfo.Script, uint64(sudtMaxCapacity)),
	}
	protocolInfo := HubProtocolInfo{
		FeeWatcher:   &MockFeeWatcher{},
		FeeStructure: &MockFeeStructure{},
	}
	setup.HubProtocol = protocolInfo
	return setup
}

func setupPaymentClient(t *testing.T, name string, config *TestConfig, d backend.Deployment, acc *ckbwallet.Account, key secp256k1.PrivateKey, w *ckbwallet.EphemeralWallet, part ckbaddr.Participant) (*client.PaymentClient, func(), error) {
	pclient, err := client.NewPaymentClient(
		name,
		Network,
		d,
		config.RPCNodeURL,
		acc,
		key,
		w,
	)
	assert.NoError(t, err, "error creating payment client for "+name)
	return pclient, func() {
		pclient.Shutdown()

	}, err
}

func (set *PaymentClientSetup) setupWalletService(t *testing.T, name string, account *ckbwallet.Account, privateKey *secp256k1.PrivateKey, network types.Network) (chproto.WalletServiceClient, *chtest.MyWalletService, func()) {
	lis := bufconn.Listen(bufSize)
	wsc := chtest.NewWalletServiceServer(name, account, privateKey, network)
	baseServer := grpc.NewServer()
	chproto.RegisterWalletServiceServer(baseServer, wsc)
	go func() {
		err := baseServer.Serve(lis)
		require.NoError(t, err, "Server exited with error for %s", name)
	}()

	conn, err := grpc.NewClient("passthrough://bufnet",
		grpc.WithContextDialer(func(ctx context.Context, s string) (net.Conn, error) {
			return lis.Dial()
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err, "Failed to dial bufnet for %s", name)

	return chproto.NewWalletServiceClient(conn), wsc, func() {
		err := lis.Close()
		if err != nil {
			log.Printf("error closing listener: %v", err)
		}
		baseServer.Stop()
	}
}
