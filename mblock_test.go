package spvwallet

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/wire"
	"testing"
)

func TestMakeMerkleParent(t *testing.T) {
	// Test same hash
	left, err := chainhash.NewHashFromStr("35be3035ce615f40af9b04124d05c64ecf14c96f37e7de02a57e4211972df04d")
	if err != nil {
		t.Error(err)
	}
	h, err := MakeMerkleParent(left, left)
	if err == nil {
		t.Error("Checking for duplicate hashes failed")
	}

	// Check left child nil
	h, err = MakeMerkleParent(nil, left)
	if err == nil {
		t.Error("Checking for nil left failed")
	}

	// Check right child nil
	h, err = MakeMerkleParent(left, nil)
	if err != nil {
		t.Error(err)
	}
	var sha [64]byte
	copy(sha[:32], left.CloneBytes()[:])
	copy(sha[32:], left.CloneBytes()[:])
	sgl := sha256.Sum256(sha[:])
	dbl := sha256.Sum256(sgl[:])
	if !bytes.Equal(dbl[:], h.CloneBytes()) {
		t.Error("Invalid hash returned when right is nil")
	}

	// Check valid hash return
	right, err := chainhash.NewHashFromStr("051b2338a496800ac09d130aee71096e13c73ccc28e83dc92d9439491d8be449")
	if err != nil {
		t.Error(err)
	}
	h, err = MakeMerkleParent(left, right)
	if err != nil {
		t.Error(err)
	}
	copy(sha[:32], left.CloneBytes()[:])
	copy(sha[32:], right.CloneBytes()[:])
	sgl = sha256.Sum256(sha[:])
	dbl = sha256.Sum256(sgl[:])
	if !bytes.Equal(dbl[:], h.CloneBytes()) {
		t.Error("Invalid hash returned")
	}
}

func TestMBolck_treeDepth(t *testing.T) {
	if treeDepth(8) != 3 {
		t.Error("treeDepth returned incorrect value")
	}
	if treeDepth(16) != 4 {
		t.Error("treeDepth returned incorrect value")
	}
	if treeDepth(64) != 6 {
		t.Error("treeDepth returned incorrect value")
	}
}

func TestMBolck_nextPowerOfTwo(t *testing.T) {
	if nextPowerOfTwo(5) != 8 {
		t.Error("treeDepth returned incorrect value")
	}
	if nextPowerOfTwo(15) != 16 {
		t.Error("treeDepth returned incorrect value")
	}
	if nextPowerOfTwo(57) != 64 {
		t.Error("treeDepth returned incorrect value")
	}
}

func TestMBlock_inDeadZone(t *testing.T) {
	// Test greater than root
	if !inDeadZone(127, 57) {
		t.Error("Failed to detect position greater than root")
	}
	// Test not in dead zone
	if inDeadZone(126, 57) {
		t.Error("Incorrectly returned in dead zone")
	}
}

func TestMBlockCheckMBlock(t *testing.T) {
	rawBlock, err := hex.DecodeString("000000204149a82a4db84c25eabdd220ae55e568f3332f9a9d6bcc21be8d010000000000f783cb176b1c29fcb191eeb7299a105fc5db9a42be7cec34d08b8d819bb64fe44d1c495d71a5021a2ad64e1e26000000077702820166697756300bb36b2268ff36d93bbe63d09d42b42c7eb52a06aa9153320007b74b0935cbd73dd85deb23a2cc2268514e72d3795b563db1f77f8503aac3690bf489db8b0f3630a0f50a6767790c6f178d1027385f14d7e70ce2622a4a125da8708c3ddfb554fd8a636152007ca6f7ad7251c2514a07ea19a3718fb6b464259f0e6b7b06e34ae8f6c2e54d4d10c603cda1d2c1ebaf093c074e5b51e3a131b237e55e259bf74174441256a61f9d62d250d06ddcec3f6f94a3f6f43e3e3e59a4fc0e7c7dc59b926c2de2f4e9176ffbf7545e17b763cdc962d829500c321002bf00")
	if err != nil {
		t.Error(err)
	}
	merkleBlock := &wire.MsgMerkleBlock{}
	r := bytes.NewReader(rawBlock)
	merkleBlock.BtcDecode(r, 70002, wire.WitnessEncoding)
	hashes, err := checkMBlock(merkleBlock)
	fmt.Printf("txnum: %d, hashnum: %d, retlen: %d\n", merkleBlock.Transactions, len(merkleBlock.Hashes), len(hashes))
	if err != nil {
		t.Error(err)
	}
	if len(hashes) != 1 {
		t.Error("Returned incorrect number of hashes")
	}

	fmt.Printf("hash is %s\n", hashes[0].String())
	//if hashes[0].String() != "652b0aa4cf4f17bdb31f7a1d308331bba91f3b3cbf8f39c9cb5e19d4015b9f01" {
	//	t.Error("Returned incorrect hash")
	//}
}
