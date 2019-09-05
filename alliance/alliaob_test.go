package alliance

import (
	"encoding/hex"
	"github.com/Zou-XueYan/spvwallet/log"
	mcommon "github.com/ontio/multi-chain/common"
	"github.com/ontio/multi-chain/smartcontract/service/native/cross_chain_manager/btc"
	sdk "github.com/ontio/ontology-go-sdk"
	"github.com/ontio/ontology-go-sdk/common"
	"testing"
	"time"
)

var (
	RpcAddr = "http://138.91.6.125:40336"
)

func TestNewObserver(t *testing.T) {
	allia := sdk.NewOntologySdk()
	allia.NewRpcClient().SetAddress(RpcAddr)
	voting := make(chan *btc.BtcProof, 10)

	NewObserver(allia, &ObConfig{
		FirstN:       10,
		LoopWaitTime: 2,
		WatchingKey:  "notifyBtcProof",
	}, voting)
}

func TestObserver_Listen(t *testing.T) {
	allia := sdk.NewOntologySdk()
	allia.NewRpcClient().SetAddress(RpcAddr)
	voting := make(chan *btc.BtcProof, 10)

	ob := NewObserver(allia, &ObConfig{
		FirstN:       10,
		LoopWaitTime: 2,
		WatchingKey:  "notifyBtcProof",
	}, voting)
	log.InitLog(2, log.Stdout)

	go ob.Listen()

	time.Sleep(time.Second * 5)
}

func TestObserver_checkEvents(t *testing.T) {
	allia := sdk.NewOntologySdk()
	allia.NewRpcClient().SetAddress(RpcAddr)
	voting := make(chan *btc.BtcProof, 10)

	ob := NewObserver(allia, &ObConfig{
		FirstN:       10,
		LoopWaitTime: 2,
		WatchingKey:  "notifyBtcProof",
	}, voting)
	log.InitLog(2, log.Stdout)

	sink := mcommon.NewZeroCopySink(nil)
	Bp1.Serialization(sink)

	events := make([]*common.SmartContactEvent, 1)
	notifys := make([]*common.NotifyEventInfo, 0)
	notifys = append(notifys, &common.NotifyEventInfo{
		ContractAddress: "1234",
		States: []interface{}{
			"notifyBtcProof",
			hex.EncodeToString(sink.Bytes()),
		},
	})

	events[0] = &common.SmartContactEvent{
		TxHash:      "123",
		State:       1,
		GasConsumed: 100,
		Notify:      notifys,
	}

	cnt := ob.checkEvents(events, 1)
	if cnt != 1 {
		t.Fatalf("wrong num: %d, should be 1", cnt)
	}
	item := <-voting
	if item.Tx == nil {
		t.Fatalf("wrong item")
	}
}
