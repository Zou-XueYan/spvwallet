package alliance

import (
	"errors"
	"fmt"
	"github.com/btcsuite/btcd/wire"
	sdk "github.com/ontio/multi-chain-go-sdk"
	"github.com/ontio/multi-chain/common"
	"github.com/ontio/multi-chain/common/password"
	"github.com/ontio/multi-chain/native/service/cross_chain_manager/btc"
	"time"
)

const (
	OP_RETURN_SCRIPT_FLAG = byte(0x66)
	BTC_CHAINID           = 0
	MIN_FEE               = 100
)

type ToVoteItem struct {
	MsgTx        *wire.MsgTx
	Height       uint32
	Proof        []byte
	BlocksToWait uint64
}

type ToSignItem struct {
	Mtx    *wire.MsgTx
	Redeem []byte
}

func ifCanResolve(paramOutput *wire.TxOut, value int64) error {
	script := paramOutput.PkScript
	if script[2] != OP_RETURN_SCRIPT_FLAG {
		return errors.New("wrong flag")
	}
	args := btc.Args{}
	err := args.Deserialization(common.NewZeroCopySource(script[3:]))
	if err != nil {
		return err
	}
	//if fee < MIN_FEE {
	//	return fmt.Errorf("transaction fee %d is less than minimum transaction fee %d", fee, MIN_FEE)
	//}
	if value < args.Fee && args.Fee >= 0 {
		return errors.New("the transfer amount cannot be less than the transaction fee")
	}
	return nil
}

func GetAccountByPassword(sdk *sdk.MultiChainSdk, path, pwd string) (*sdk.Account, error) {
	wallet, err := sdk.OpenWallet(path)
	if err != nil {
		return nil, fmt.Errorf("open wallet error: %v", err)
	}
	pwdb := []byte{}
	if pwd == "" {
		pwdb, err = password.GetPassword()
		if err != nil {
			return nil, fmt.Errorf("getPassword error: %v", err)
		}
	} else {
		pwdb = []byte(pwd)
	}
	user, err := wallet.GetDefaultAccount(pwdb)
	if err != nil {
		return nil, fmt.Errorf("getDefaultAccount error: %v", err)
	}
	return user, nil
}

type LessConfirmationError struct {
	Err error
}

func (err LessConfirmationError) Error() string {
	return err.String()
}

func (err *LessConfirmationError) String() string {
	return err.Err.Error()
}

func wait(dura time.Duration) {
	t := time.NewTimer(dura)
	<-t.C
	t.Stop()
}