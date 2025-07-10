module perun.network/vc-hub-service

go 1.24.0

require (
	google.golang.org/grpc v1.74.2
	google.golang.org/protobuf v1.36.6
	perun.network/go-perun v0.12.1-0.20250415090022-4d68d2869b94
	perun.network/perun-ckb-backend v0.2.0
)

require (
	github.com/Pilatuz/bigz v1.2.1 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/decred/dcrd/dcrec/secp256k1/v4 v4.2.0 // indirect
	github.com/ethereum/go-ethereum v1.13.10 // indirect
	github.com/holiman/uint256 v1.2.4 // indirect
	github.com/minio/blake2b-simd v0.0.0-20160723061019-3f5f724cb5b1 // indirect
	github.com/nervosnetwork/ckb-sdk-go/v2 v2.2.0 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/sirupsen/logrus v1.9.0 // indirect
	github.com/stretchr/testify v1.8.4 // indirect
	golang.org/x/crypto v0.38.0 // indirect
	golang.org/x/net v0.40.0 // indirect
	golang.org/x/sync v0.14.0 // indirect
	golang.org/x/sys v0.33.0 // indirect
	golang.org/x/text v0.25.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20250528174236-200df99c418a // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
	polycry.pt/poly-go v0.0.0-20220301085937-fb9d71b45a37 // indirect
)

replace github.com/nervosnetwork/ckb-sdk-go/v2 v2.2.0 => github.com/perun-network/ckb-sdk-go/v2 v2.2.1-0.20250414095541-e6244b21519c
