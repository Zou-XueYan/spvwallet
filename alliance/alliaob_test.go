package alliance

import (
	"encoding/hex"
	"fmt"
	"github.com/Zou-XueYan/spvwallet/log"
	sdk "github.com/ontio/multi-chain-go-sdk"
	"github.com/ontio/multi-chain-go-sdk/common"
	mcommon "github.com/ontio/multi-chain/common"
	"github.com/ontio/multi-chain/native/service/cross_chain_manager/btc"
	"testing"
	"time"
)

var (
	RpcAddr = "http://138.91.6.125:40336"
)

func TestNewObserver(t *testing.T) {
	allia := sdk.NewMultiChainSdk()
	allia.NewRpcClient().SetAddress(RpcAddr)
	voting := make(chan *btc.BtcProof, 10)

	txc := make(chan *ToSignItem)
	NewObserver(allia, &ObConfig{
		FirstN:       10,
		LoopWaitTime: 2,
		WatchingKey:  "notifyBtcProof",
	}, voting, txc)
}

func TestObserver_Listen(t *testing.T) {
	allia := sdk.NewMultiChainSdk()
	allia.NewRpcClient().SetAddress(RpcAddr)
	voting := make(chan *btc.BtcProof, 10)

	txc := make(chan *ToSignItem)
	ob := NewObserver(allia, &ObConfig{
		FirstN:       10,
		LoopWaitTime: 2,
		WatchingKey:  "notifyBtcProof",
	}, voting, txc)
	log.InitLog(2, log.Stdout)

	go ob.Listen()

	time.Sleep(time.Second * 5)
}

func TestObserver_checkEvents(t *testing.T) {
	allia := sdk.NewMultiChainSdk()
	allia.NewRpcClient().SetAddress(RpcAddr)
	voting := make(chan *btc.BtcProof, 10)

	txc := make(chan *ToSignItem, 10)
	ob := NewObserver(allia, &ObConfig{
		FirstN:            10,
		LoopWaitTime:      2,
		WatchingKey:       "notifyBtcProof",
		WatchingMakeTxKey: "btcTxToRelay",
	}, voting, txc)
	log.InitLog(2, log.Stdout)

	sink := mcommon.NewZeroCopySink(nil)
	Bp1.Serialization(sink)

	events := make([]*common.SmartContactEvent, 1)
	notifys := make([]*common.NotifyEventInfo, 0)
	notifys = append(notifys, &common.NotifyEventInfo{
		ContractAddress: "1234",
		States: []interface{}{
			"notifyBtcProof",
			"txid",
			hex.EncodeToString(sink.Bytes()),
		},
	}, &common.NotifyEventInfo{
		ContractAddress: "1234",
		States: []interface{}{
			"btcTxToRelay",
			usignedTx,
			redeem,
		},
	})

	events[0] = &common.SmartContactEvent{
		TxHash:      "123",
		State:       1,
		GasConsumed: 100,
		Notify:      notifys,
	}

	cnt, cntTx := ob.checkEvents(events, 1)
	if cnt != 1 {
		t.Fatalf("wrong num: %d, should be 1", cnt)
	}
	if cntTx != 1 {
		t.Fatalf("wrong tx num: %d, should be 1", cntTx)
	}
	item := <-voting
	if item.Tx == nil {
		t.Fatalf("wrong item")
	}
	txItem := <-txc
	if txItem.Mtx.TxHash().String() == "" {
		t.Fatalf("wrong tx item")
	}
	fmt.Println(txItem.Mtx.TxHash().String())
}

func TestGetAccountByPassword(t *testing.T) {
	allia := sdk.NewMultiChainSdk()
	acct, err := GetAccountByPassword(allia, "./wallet.dat", "1")
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println(acct.Address.ToBase58())
}
