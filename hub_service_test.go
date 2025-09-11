package main

import (
	"log"
	"testing"

	"perun.network/vc-hub-service/test"
)

func TestHappyCase(t *testing.T) {
	testConfig := test.DevnetConfig()
	test.NewTestSetup(t, testConfig)
	log.Println("Yayy")
}
