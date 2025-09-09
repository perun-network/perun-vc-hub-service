package wallet

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/decred/dcrd/dcrec/secp256k1/v4"
	address2 "github.com/nervosnetwork/ckb-sdk-go/v2/address"
	"github.com/nervosnetwork/ckb-sdk-go/v2/transaction"
	"github.com/nervosnetwork/ckb-sdk-go/v2/types"
	"perun.network/perun-ckb-backend/wallet/address"
	"perun.network/vc-hub-service/rpc/proto"
)

type RemoteSigner struct {
	wcs         proto.WalletServiceClient
	addr        address2.Address
	participant *address.Participant
}

func NewRemoteSigner(wcs proto.WalletServiceClient, addr address2.Address, participant *address.Participant) *RemoteSigner {
	return &RemoteSigner{
		wcs:  wcs,
		addr: addr,
	}
}

func (s *RemoteSigner) PublicKey() *secp256k1.PublicKey {
	if s.participant == nil {
		log.Panic("RemoteSigner participant is nil")
	}
	return s.participant.PubKey
}

func (s RemoteSigner) SignTransaction(tx *transaction.TransactionWithScriptGroups) (*types.Transaction, error) {
	scriptBytes, err := json.Marshal(s.addr.Script)
	if err != nil {
		return nil, err
	}

	txBytes, err := json.Marshal(tx)

	if err != nil {
		return nil, err
	}
	req := &proto.SignTransactionRequest{
		Identifier:  scriptBytes, // TODO: Maybe encode network also?
		Transaction: txBytes,
	}
	resp, err := s.wcs.SignTransaction(context.TODO(), req)
	if err != nil {
		return nil, err
	}
	if rej := resp.GetRejected(); rej != nil {
		return nil, fmt.Errorf("transaction signing failed: %s", rej.Reason)
	}

	var signedTx types.Transaction
	signedTxBytes := resp.GetTransaction()
	if err = json.Unmarshal(signedTxBytes, &signedTx); err != nil {
		return nil, err
	}
	return &signedTx, nil
}

func (s RemoteSigner) Address() address2.Address {
	return s.addr
}
