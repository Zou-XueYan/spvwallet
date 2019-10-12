package alliance

import (
	"bytes"
	"encoding/hex"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/wire"
	sdk "github.com/ontio/multi-chain-go-sdk"
	"testing"
	"time"
)

var (
	privk     = "cTqbqa1YqCf4BaQTwYDGsPAB4VmWKUU67G5S1EtrHSWNRwY6QSag"
	usignedTx = "01000000015ef067df7af576fa5b43bb7e99846c970af7e998cf060c9942920883a515cc6c0000000000ffffffff01401f00000000000017a91487a9652e9b396545598c0fc72cb5a98848bf93d38700000000"
	redeem    = "5521023ac710e73e1410718530b2686ce47f12fa3c470a9eb6085976b70b01c64c9f732102c9dc4d8f419e325bbef0fe039ed6feaf2079a2ef7b27336ddb79be2ea6e334bf2102eac939f2f0873894d8bf0ef2f8bbdd32e4290cbf9632b59dee743529c0af9e802103378b4a3854c88cca8bfed2558e9875a144521df4a75ab37a206049ccef12be692103495a81957ce65e3359c114e6c2fe9f97568be491e3f24d6fa66cc542e360cd662102d43e29299971e802160a92cfcd4037e8ae83fb8f6af138684bebdc5686f3b9db21031e415c04cbc9b81fbee6e04d8c902e8f61109a2c9883a959ba528c52698c055a57ae"
	sig1      = "30440220328fcf07c207b20309c2f42427079592771a1fe63e7196e476c258b32950cc0e022016207f8b39b6af70dd789524cb6bb30927f6e493f798ec29e742b82c119ab2da01"
)

func TestNewSigner(t *testing.T) {
	txchan := make(chan *ToSignItem, 10)
	allia := sdk.NewMultiChainSdk()
	allia.NewRpcClient().SetAddress(RpcAddr)
	acct, err := GetAccountByPassword(allia, "../cmd/lightcli/wallet.dat", "passwordtest")
	if err != nil {
		t.Fatal(err)
	}
	_, err = NewSigner(privk, txchan, acct, 0, 20000, allia, &chaincfg.TestNet3Params)
	if err != nil {
		t.Fatal(err)
	}
}

func TestSigner_Signing(t *testing.T) {
	txchan := make(chan *ToSignItem, 10)
	allia := sdk.NewMultiChainSdk()
	allia.NewRpcClient().SetAddress(RpcAddr)
	acct, err := GetAccountByPassword(allia, "../cmd/lightcli/wallet.dat", "passwordtest")
	if err != nil {
		t.Fatal(err)
	}
	signer, err := NewSigner(privk, txchan, acct, 0, 20000, allia, &chaincfg.TestNet3Params)
	if err != nil {
		t.Fatal(err)
	}

	go signer.Signing()
	time.Sleep(2 * time.Second)

	mtx := wire.NewMsgTx(wire.TxVersion)
	buf, _ := hex.DecodeString(usignedTx)
	mtx.BtcDecode(bytes.NewBuffer(buf), wire.ProtocolVersion, wire.LatestEncoding)

	r, _ := hex.DecodeString(redeem)

	txchan <- &ToSignItem{
		Mtx:    mtx,
		Redeem: r,
	}

	time.Sleep(2 * time.Second)
}

func TestSigner_GetSigs(t *testing.T) {
	txchan := make(chan *ToSignItem, 10)
	allia := sdk.NewMultiChainSdk()
	allia.NewRpcClient().SetAddress(RpcAddr)
	acct, err := GetAccountByPassword(allia, "../cmd/lightcli/wallet.dat", "passwordtest")
	if err != nil {
		t.Fatal(err)
	}
	signer, err := NewSigner(privk, txchan, acct, 0, 20000, allia, &chaincfg.TestNet3Params)
	if err != nil {
		t.Fatal(err)
	}

	mtx := wire.NewMsgTx(wire.TxVersion)
	buf, _ := hex.DecodeString(usignedTx)
	mtx.BtcDecode(bytes.NewBuffer(buf), wire.ProtocolVersion, wire.LatestEncoding)

	r, _ := hex.DecodeString(redeem)

	sigs, err := signer.getSigs(mtx, r)
	if err != nil {
		t.Fatal(err)
	}

	if hex.EncodeToString(sigs[0]) != sig1 {
		t.Fatal("not equal")
	}
}
