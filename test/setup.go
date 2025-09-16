package test

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"testing"

	"github.com/decred/dcrd/dcrec/secp256k1/v4"
	"github.com/nervosnetwork/ckb-sdk-go/v2/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
	"perun.network/vc-hub-service/protocol"
	"perun.network/vc-hub-service/rpc/proto"
	"perun.network/vc-hub-service/service"
	"perun.network/vc-hub-service/test/deployment"
	"polycry.pt/poly-go/sortedkv"
	"polycry.pt/poly-go/sortedkv/memorydb"

	"perun.network/go-perun/channel/persistence"

	"perun.network/perun-ckb-backend/backend"
	"perun.network/perun-ckb-backend/channel/asset"
	ckbwallet "perun.network/perun-ckb-backend/wallet"
	ckbaddr "perun.network/perun-ckb-backend/wallet/address"
	"perun.network/perun-ckb-backend/wallet/external"

	chproto "perun.network/channel-service/rpc/proto"
	chservice "perun.network/channel-service/service"
	chtest "perun.network/channel-service/service/test"
	chwallet "perun.network/channel-service/wallet"
)

const (
	devNetURL              = "http://localhost:8114"
	testNetURL             = "https://testnet.ckbapp.dev/"
	devNetDir              = "devnet"          // DevNetDir is the directory where the devnet configuration is located.
	testNetDir             = "testnet"         // TestNetDir is the directory where the testnet configuration is located.
	Network                = types.NetworkTest // Network is the network used for testing.
	bufSize                = 1024 * 1024
	sudtMaxCapacity        = 200_00_000_000 // 200 ckb
	SUDTOwnerLockArgFile   = "accounts/sudt-owner-lock-hash.txt"
	ContractMigrationsPath = "contracts/migrations/dev/"
	SystemScriptsDir       = "system_scripts"
	AlicePKFile            = "accounts/alice.pk"
	BobPKFile              = "accounts/bob.pk"
	HubOwnerPKFile         = "accounts/ingrid.pk"
)

type HubWalletInfo struct {
	WalletService *chtest.MyWalletService
	CleanupFunc   func()
	WSClient      chproto.WalletServiceClient
}

type HubServiceInfo struct {
	HubService  *service.HubService
	CleanupFunc func()
	HubClient   proto.VCHubServiceClient
	User        *service.User
}

type HubProtocolInfo struct {
	FeeWatcher   protocol.Watcher
	FeeStructure protocol.FeeStructure
}

// Setup contains all the necessary information for testing.
type Setup struct {
	t                          *testing.T
	Deployment                 backend.Deployment
	SUDTInfo                   deployment.SUDTInfo
	WalletAccs                 []*ckbwallet.Account
	AccPersistRestorers        []persistence.PersistRestorer
	Asset                      asset.Asset
	SudtAsset                  asset.Asset
	AccKeys                    []*secp256k1.PrivateKey
	Participants               []ckbaddr.Participant
	WalletServiceClients       []chproto.WalletServiceClient
	WscCleanupFuncs            []func()
	WalletServices             []*chtest.MyWalletService
	ChannelServiceClients      []chproto.ChannelServiceClient
	ChannelServices            []*chservice.ChannelService
	ChannelServiceCleanupFuncs []func()
	Databases                  []*sortedkv.Database
	HubWallet                  HubWalletInfo
	HubService                 HubServiceInfo
	HubProtocol                HubProtocolInfo
	Users                      []*chservice.User
}

// NewTestSetup creates a new setup for testing.
func NewTestSetup(t *testing.T, testConfig *TestConfig) *Setup {
	setup := &Setup{}
	setup.t = t
	sudtOwnerLockArg, err := parseSUDTOwnerLockArg(testConfig.NetworkDirectory + "/" + SUDTOwnerLockArgFile)
	require.NoError(t, err, "error getting SUDT owner lock arg")

	d, sudtInfo, err := deployment.GetDeployment(testConfig.NetworkDirectory+"/"+ContractMigrationsPath, testConfig.NetworkDirectory+"/"+SystemScriptsDir, sudtOwnerLockArg)
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

	setup.WalletAccs = []*ckbwallet.Account{aliceAccount, bobAccount, hubAccount}
	setup.AccKeys = []*secp256k1.PrivateKey{alicePrivateKey, bobPrivateKey, hubOwnerPrivateKey}

	aliceWSC, aliceWSCCleanup := setup.setupWalletService(t, "alice", aliceAccount, alicePrivateKey, Network)
	bobWSC, bobWSCCleanup := setup.setupWalletService(t, "bob", bobAccount, bobPrivateKey, Network)
	hubWSC, hubWSCCleanup := setup.setupWalletService(t, "hub", hubAccount, hubOwnerPrivateKey, Network)
	setup.HubWallet = HubWalletInfo{
		CleanupFunc:   hubWSCCleanup,
		WSClient:      hubWSC,
		WalletService: setup.WalletServices[2],
	}

	setup.WscCleanupFuncs = []func(){aliceWSCCleanup, bobWSCCleanup}
	setup.WalletServiceClients = []chproto.WalletServiceClient{aliceWSC, bobWSC}

	aliceDB := memorydb.NewDatabase()
	bobDB := memorydb.NewDatabase()
	setup.Databases = []*sortedkv.Database{&aliceDB, &bobDB}

	aliceHubClient, aliceCS, aliceCSCleanup := setupChannelService(t, "alice", aliceWSC, Network, testConfig.RPCNodeURL, d, nil, &aliceDB)
	bobHSClient, bobCS, bobCSCleanup := setupChannelService(t, "bob", bobWSC, Network, testConfig.RPCNodeURL, d, nil, &bobDB)
	setup.ChannelServiceClients = []chproto.ChannelServiceClient{aliceHubClient, bobHSClient}
	setup.ChannelServices = []*chservice.ChannelService{aliceCS, bobCS}
	setup.ChannelServiceCleanupFuncs = []func(){aliceCSCleanup, bobCSCleanup}
	log.Printf("Participants: %v", parts)

	setup.Users = make([]*chservice.User, 2)
	log.Println("Initializing Users")
	aliceUser, err := aliceCS.InitializeUser(parts[0], aliceWSC, external.NewWallet(chwallet.NewExternalClient(aliceWSC)))
	assert.NoError(t, err, "error initializing alice user")
	setup.Users[0] = aliceUser
	log.Println("Initialized alice user")
	bobUser, err := bobCS.InitializeUser(parts[1], bobWSC, external.NewWallet(chwallet.NewExternalClient(bobWSC)))
	assert.NoError(t, err, "error initializing bob user")
	setup.Users[1] = bobUser
	log.Println("Initialized bob user")
	// // Initialize Normal Users
	// for i, part := range parts[:2] {
	// 	if i == 0 {
	// 		user, err := aliceCS.InitializeUser(part, aliceWSC, external.NewWallet(chwallet.NewExternalClient(aliceWSC)))
	// 		setup.Users[i] = user
	// 	} else {
	// 		user, err := bobCS.InitializeUser(part, bobWSC, external.NewWallet(chwallet.NewExternalClient(bobWSC)))
	// 		setup.Users[i] = user
	// 	}
	// 	require.NoError(t, err, "error initializing user %d", i)
	// }

	//Initialize Hub Service and User
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

func setupChannelService(t *testing.T, name string, wsc chproto.WalletServiceClient, network types.Network, rpcNodeUrl string, d backend.Deployment, addrResolver service.AddressResolver, db *sortedkv.Database) (chproto.ChannelServiceClient, *chservice.ChannelService, func()) {
	cs, err := chservice.NewChannelService(wsc, network, rpcNodeUrl, d, addrResolver, *db)
	require.NoError(t, err, "error setting up channel service for %s", name)
	lis := bufconn.Listen(bufSize)
	baseServer := grpc.NewServer()
	log.Printf("Registering channel service server for %s", name)
	chproto.RegisterChannelServiceServer(baseServer, cs)
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

	return chproto.NewChannelServiceClient(conn), cs, func() {
		err := cs.Close()
		if err != nil {
			log.Printf("error closing channel service: %v", err)
		}
		err = lis.Close()
		if err != nil {
			log.Printf("error closing listener: %v", err)
		}
		baseServer.Stop()
	}
}

func setupHubService(t *testing.T, name string, wsc chproto.WalletServiceClient, network types.Network, rpcNodeUrl string, d backend.Deployment, addrResolver service.AddressResolver, part ckbaddr.Participant) (proto.VCHubServiceClient, *service.HubService, func()) {
	hs, err := service.NewHubService(wsc, network, rpcNodeUrl, d, addrResolver, part)
	require.NoError(t, err, "error setting up hub service for %s", name)
	lis := bufconn.Listen(bufSize)
	baseServer := grpc.NewServer()
	log.Printf("Registering hub service server for %s", name)
	proto.RegisterVCHubServiceServer(baseServer, hs)
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

	return proto.NewVCHubServiceClient(conn), hs, func() {
		err := hs.Close()
		if err != nil {
			log.Printf("error closing hub service: %v", err)
		}
		err = lis.Close()
		if err != nil {
			log.Printf("error closing listener: %v", err)
		}
		baseServer.Stop()
	}
}

func (set *Setup) setupWalletService(t *testing.T, name string, account *ckbwallet.Account, privateKey *secp256k1.PrivateKey, network types.Network) (chproto.WalletServiceClient, func()) {
	lis := bufconn.Listen(bufSize)
	wsc := chtest.NewWalletServiceServer(name, account, privateKey, network)
	set.WalletServices = append(set.WalletServices, wsc)
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

	return chproto.NewWalletServiceClient(conn), func() {
		err := lis.Close()
		if err != nil {
			log.Printf("error closing listener: %v", err)
		}
		baseServer.Stop()
	}
}

// MakeParticipants creates a list of participants from a list of public keys.
func MakeParticipants(pks []*secp256k1.PublicKey) ([]ckbaddr.Participant, error) {
	parts := make([]ckbaddr.Participant, len(pks))
	for i := range pks {
		part, err := ckbaddr.NewDefaultParticipant(pks[i])
		if err != nil {
			return nil, fmt.Errorf("unable to create participant: %w", err)
		}
		parts[i] = *part
	}
	return parts, nil
}

func parseSUDTOwnerLockArg(pathToSUDTOwnerLockArg string) (string, error) {
	b, err := os.ReadFile(pathToSUDTOwnerLockArg)
	if err != nil {
		return "", fmt.Errorf("reading sudt owner lock arg from file: %w", err)
	}
	sudtOwnerLockArg := string(b)
	if sudtOwnerLockArg == "" {
		return "", errors.New("sudt owner lock arg not found in file")
	}
	return sudtOwnerLockArg, nil
}
