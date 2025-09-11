package test

import (
	"path/filepath"
	"runtime"

	"github.com/nervosnetwork/ckb-sdk-go/v2/types"
)

type TestConfig struct {
	RPCNodeURL       string
	CkbNetworkType   types.Network
	NetworkDirectory string
}

func TestnetConfig() *TestConfig {
	return &TestConfig{
		RPCNodeURL:       testNetURL,
		CkbNetworkType:   types.NetworkTest,
		NetworkDirectory: resolveNetworkDirectory(testNetDir),
	}
}

func DevnetConfig() *TestConfig {
	return &TestConfig{
		RPCNodeURL:       devNetURL,
		CkbNetworkType:   types.NetworkTest,
		NetworkDirectory: resolveNetworkDirectory(devNetDir),
	}
}

// resolveDevNetDir makes path relative to this package (test/).
func resolveNetworkDirectory(relativePath string) string {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		panic("cannot determine caller")
	}
	// test/ + ../devnet (repo root/devnet)
	return filepath.Clean(filepath.Join(filepath.Dir(file), relativePath))
}
