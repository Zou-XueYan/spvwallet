package alliance

import (
	"encoding/hex"
	"fmt"
	"github.com/btcsuite/btcd/chaincfg"
	sdk "github.com/ontio/multi-chain-go-sdk"
	"github.com/ontio/multi-chain/common"
	"github.com/ontio/multi-chain/native/service/cross_chain_manager/btc"
	"github.com/ontio/spvclient"
	"os"
	"strconv"
	"testing"
	"time"
)

func TestNewVoter(t *testing.T) {
	allia := sdk.NewMultiChainSdk()
	allia.NewRpcClient().SetAddress(RpcAddr)
	voting := make(chan *btc.BtcProof, 10)

	acct, err := GetAccountByPassword(allia, "../cmd/lightcli/wallet.dat", "passwordtest")
	if err != nil {
		t.Fatalf("Failed to get acct: %v", err)
	}

	conf := spvclient.NewDefaultConfig()
	conf.RepoPath = "./"
	conf.Params = &chaincfg.TestNet3Params
	if err != nil {
		t.Fatalf("failed to create sqlite db: %v", err)
	}
	wallet, _ := spvclient.NewSPVWallet(conf)

	redeem, _ := hex.DecodeString("5521023ac710e73e1410718530b2686ce47f12fa3c470a9eb6085976b70b01c64c9f732102c9dc4d8f419e325bbef0fe039ed6feaf2079a2ef7b27336ddb79be2ea6e334bf2102eac939f2f0873894d8bf0ef2f8bbdd32e4290cbf9632b59dee743529c0af9e802103378b4a3854c88cca8bfed2558e9875a144521df4a75ab37a206049ccef12be692103495a81957ce65e3359c114e6c2fe9f97568be491e3f24d6fa66cc542e360cd662102d43e29299971e802160a92cfcd4037e8ae83fb8f6af138684bebdc5686f3b9db21031e415c04cbc9b81fbee6e04d8c902e8f61109a2c9883a959ba528c52698c055a57ae")
	_, err = NewVoter(allia, voting, wallet, redeem, acct,  nil, 6)
	if err != nil {
		t.Fatalf("failed to new voter: %v", err)
	}

	defer func() {
		os.RemoveAll("./waiting.bin")
		os.RemoveAll("./headers.bin")
		os.RemoveAll("./wallet.db")
	}()
}

// if you want to pass the test, you need to sync enough
// headers and broadcast a cross chain transaction. not a good test case.
func TestVoter_Vote(t *testing.T) {
	allia := sdk.NewMultiChainSdk()
	allia.NewRpcClient().SetAddress(RpcAddr)
	voting := make(chan *btc.BtcProof, 10)

	acct, err := GetAccountByPassword(allia, "../cmd/lightcli/wallet.dat", "passwordtest")
	if err != nil {
		t.Fatalf("Failed to get acct: %v", err)
	}

	conf := spvclient.NewDefaultConfig()
	conf.RepoPath = "./"
	conf.Params = &chaincfg.TestNet3Params
	if err != nil {
		t.Fatalf("failed to create sqlite db: %v", err)
	}
	wallet, _ := spvclient.NewSPVWallet(conf)
	redeem, _ := hex.DecodeString("5521023ac710e73e1410718530b2686ce47f12fa3c470a9eb6085976b70b01c64c9f732102c9dc4d8f419e325bbef0fe039ed6feaf2079a2ef7b27336ddb79be2ea6e334bf2102eac939f2f0873894d8bf0ef2f8bbdd32e4290cbf9632b59dee743529c0af9e802103378b4a3854c88cca8bfed2558e9875a144521df4a75ab37a206049ccef12be692103495a81957ce65e3359c114e6c2fe9f97568be491e3f24d6fa66cc542e360cd662102d43e29299971e802160a92cfcd4037e8ae83fb8f6af138684bebdc5686f3b9db21031e415c04cbc9b81fbee6e04d8c902e8f61109a2c9883a959ba528c52698c055a57ae")

	wallet.Start()
	defer func() {
		wallet.Close()
		os.RemoveAll("./peers.json")
		os.RemoveAll("./waiting.bin")
		os.RemoveAll("./headers.bin")
		os.RemoveAll("./wallet.db")
	}()

	v, err := NewVoter(allia, voting, wallet, redeem, acct,  nil, 6)
	if err != nil {
		t.Fatalf("failed to new voter: %v", err)
	}

	go v.Vote()
	go v.WaitingRetry()

	sink := common.NewZeroCopySink(nil)
	Bp1.Serialization(sink)

	go func() {
		for {
			voting <- Bp1
			time.Sleep(2 * time.Second)
		}
	}()

	time.Sleep(time.Second * 10)
}

func TestSth(t *testing.T) {
	for i := 0; i < 2; i++ {
		a := "sb" + strconv.Itoa(i)
		go func() {
			time.Sleep(2 * time.Second)
			fmt.Println(a)
		}()
	}

	fmt.Println("Start waiting")
	time.Sleep(5 * time.Second)
}
