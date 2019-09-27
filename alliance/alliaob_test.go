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

	quit := make(chan struct{})
	NewObserver(allia, &ObConfig{
		FirstN:       10,
		LoopWaitTime: 2,
		WatchingKey:  "notifyBtcProof",
	}, voting, quit)
}

func TestObserver_Listen(t *testing.T) {
	allia := sdk.NewMultiChainSdk()
	allia.NewRpcClient().SetAddress(RpcAddr)
	voting := make(chan *btc.BtcProof, 10)

	quit := make(chan struct{})
	ob := NewObserver(allia, &ObConfig{
		FirstN:       10,
		LoopWaitTime: 2,
		WatchingKey:  "notifyBtcProof",
	}, voting, quit)
	log.InitLog(2, log.Stdout)

	go ob.Listen()

	time.Sleep(time.Second * 5)
}

func TestObserver_checkEvents(t *testing.T) {
	allia := sdk.NewMultiChainSdk()
	allia.NewRpcClient().SetAddress(RpcAddr)
	voting := make(chan *btc.BtcProof, 10)
	quit := make(chan struct{})

	ob := NewObserver(allia, &ObConfig{
		FirstN:       10,
		LoopWaitTime: 2,
		WatchingKey:  "notifyBtcProof",
	}, voting, quit)
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

func TestSth1(t *testing.T) {
	allia := sdk.NewMultiChainSdk()
	allia.NewRpcClient().SetAddress("http://192.168.3.144:20336")

	acct, err := GetAccountByPassword(allia, "/Users/zou/go/src/github.com/Zou-XueYan/spvwallet/cmd/lightcli/wallet.dat", "passwordtest")
	if err != nil {
		t.Errorf("GetAccountByPassword failed: %v", err)
	}

	addr58 := acct.Address.ToBase58()
	//fmt.Printf("addr from wallet: %s\n", addr58)

	tx, err := allia.Native.Ccm.NewVoteTransaction(0, addr58, "8d86a6ab2c2f34d0a8ae9a90af2c82d12ab242914403e133a79209744ccf7e46")
	if err != nil {
		t.Fatalf("failed to new vote tx: %v", err)
	}
	err = allia.SignToTransaction(tx, acct)
	if err != nil {
		t.Fatalf("failed to sign: %v", err)
	}

	s, err := allia.SendTransaction(tx)
	fmt.Printf("state is %d, error is %v\n", s, err)

	//address, err := mcommon.AddressFromBase58(addr58)
	//if err != nil {
	//	t.Fatalf("failed to get addr: %v", err)
	//}

	//serv := native.NewNativeService(nil, tx, 0, 0, mcommon.Uint256{}, 0, nil, false, nil)
	//fmt.Printf("check witness: %v\n", serv.CheckWitness(address))


	//addrs, err := tx.GetSignatureAddresses()
	//if err != nil {
	//	t.Fatalf("failed to get sig addrs")
	//}

	//str := "51f19e6af2eead230e0f15490069e45122f7171e"
	//addrr, _ := mcommon.AddressFromHexString(str)
	//fmt.Println(addrr.ToBase58())

	//fmt.Println(address.ToHexString())
	//
	//for _, a := range addrs {
	//	if a == address {
	//		fmt.Println("got!!!")
	//		//return
	//	}
	//}
	//fmt.Printf("no!!!!\n")

	//var sink = new(mcommon.ZeroCopySink)
	//err = tx.Serialization(sink) // SDK的序列化
	//if err != nil {
	//	t.Fatalf("serialize error:%s", err)
	//}
	//
	//newtx, err := types.TransactionFromRawBytes(sink.Bytes()) // multi-chain 的反序列化
	//addresses, err := newtx.GetSignatureAddresses()
	//fmt.Printf("GetSignatureAddresses: %d, len of sigs: %d \n", len(addresses), len(newtx.Sigs))
}