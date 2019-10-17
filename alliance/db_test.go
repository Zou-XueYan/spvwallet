package alliance

import (
	"bytes"
	"encoding/hex"
	"github.com/btcsuite/btcd/wire"
	"github.com/ontio/multi-chain/native/service/cross_chain_manager/btc"
	"os"
	"testing"
)

var (
	RawTxStr = "010000000162148b88c5fd22eb3ae97c650f9638a749c7b31cce84687603cde618609d2c0b020000006b48304502210086f3948f23da16275d804dafeb576cc6ecbd8e1ee50012f8b523148130d37846022015a313dee224fb971b75a4c395726d7e471d45c272982bc1bbe04426d89993ea012103128a2c4525179e47f38cf3fefca37a61548ca4610255b3fb4ee86de2d3e80c0fffffffff03102700000000000017a91487a9652e9b396545598c0fc72cb5a98848bf93d3870000000000000000276a2566000000000000000200000000000000000a7714eb5a0b4f369bd080bb4cd30e2d4d35f44c00710200000000001976a91428d2e8cee08857f569e5a1b147c5d5e87339e08188ac00000000"
	Proof    = "000040202afc0fedbeb00d166634257e563f6cf74c458d876ca7bd9db801000000000000e95b4b17d1ad5e27ed395a900e708e3a091e22632318cdc49a98d07e70739c445f33675d31f7011a9b129d340e00000005c99aefe1d8373df4c5ef1486dd59391cb62ae52f6c20d0ac051e656ca7a9ac3762c1687386aa9a88b58a00416f72cacbdfb9d2ae9f0e0f16c2bf2dc7ba6c84295fe1a969548ddce398c9c880f02af141437053f7f1d913f93fc176beac6173c82b61dc50c63375da1922d1e5196304b66e3df3f3a78fba5e40708d5417572d130c32a0d5f59eddaee756eca9381824ea95985b3e9cc4b91b30ed95000e488193023b00"
	Height   = uint32(1151182) //uint32(1576318)

	Bp1 *btc.BtcProof
)

func init() {
	proof, _ := hex.DecodeString(Proof)
	tx, _ := hex.DecodeString(RawTxStr)

	Bp1 = &btc.BtcProof{
		Proof:        proof,
		Tx:           tx,
		Height:       Height,
		BlocksToWait: 6,
	}
}

func TestNewWaitingDB(t *testing.T) {
	_, err := NewWaitingDB("")
	if err != nil {
		t.Fatalf("Failed to new a db: %v", err)
	}
	defer os.RemoveAll("./waiting.bin")
	_, err = os.Stat("./waiting.bin")
	if err != nil && !os.IsExist(err) {
		t.Fatalf("Can't find waiting.bin: %v", err)
	}
}

func TestWaitingDB_Put(t *testing.T) {
	db, err := NewWaitingDB("")
	if err != nil {
		t.Fatalf("Failed to new a db: %v", err)
	}
	defer os.RemoveAll("./waiting.bin")
	mtx := wire.NewMsgTx(wire.TxVersion)
	err = mtx.BtcDecode(bytes.NewBuffer(Bp1.Tx), wire.ProtocolVersion, wire.LatestEncoding)
	if err != nil {
		t.Fatalf("Failed to decode tx: %v", err)
	}
	txid := mtx.TxHash()
	err = db.Put(txid[:], Bp1)
	if err != nil {
		t.Fatalf("Failed to put: %v", err)
	}

	p, err := db.Get(txid[:])
	if err != nil {
		t.Fatalf("Failed to get %v", err)
	}
	if !bytes.Equal(p.Tx, Bp1.Tx) {
		t.Fatal("not equal!")
	}

}

func TestWaitingDB_GetUnderHeightAndDelete(t *testing.T) {
	db, err := NewWaitingDB("")
	if err != nil {
		t.Fatalf("Failed to new a db: %v", err)
	}
	defer os.RemoveAll("./waiting.bin")

	mtx := wire.NewMsgTx(wire.TxVersion)
	err = mtx.BtcDecode(bytes.NewBuffer(Bp1.Tx), wire.ProtocolVersion, wire.LatestEncoding)
	if err != nil {
		t.Fatalf("Failed to decode tx: %v", err)
	}

	txid := mtx.TxHash()
	err = db.Put(txid[:], Bp1)
	if err != nil {
		t.Fatalf("Failed to put: %v", err)
	}

	arr, _, err := db.GetUnderHeightAndDelete(Height)
	if err != nil {
		t.Fatalf("Failed to get: %v", err)
	}
	if len(arr) != 1 {
		t.Fatalf("Wrong length: %d", len(arr))
	}
	if !bytes.Equal(arr[0].Tx, Bp1.Tx) {
		t.Fatal("not equal!")
	}
}

func TestWaitingDB_MarkVotedTx(t *testing.T) {
	db, err := NewWaitingDB("")
	if err != nil {
		t.Fatalf("Failed to new a db: %v", err)
	}
	defer os.RemoveAll("./waiting.bin")

	err = db.MarkVotedTx([]byte("123"))
	if err != nil {
		t.Fatalf("Failed to mark: %v", err)
	}

	if !db.CheckIfVoted([]byte("123")) {
		t.Fatal("not marked!")
	}
	if db.CheckIfVoted([]byte("1234")) {
		t.Fatal("not marked!")
	}
}
