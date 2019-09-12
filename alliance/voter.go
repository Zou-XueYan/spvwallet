package alliance

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/Zou-XueYan/spvwallet"
	"github.com/Zou-XueYan/spvwallet/log"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"
	"github.com/btcsuite/btcutil"
	wire_bch "github.com/gcash/bchd/wire"
	"github.com/gcash/bchutil/merkleblock"
	"github.com/ontio/multi-chain/native/service/cross_chain_manager/btc"
	"github.com/ontio/multi-chain/native/service/cross_chain_manager/common"
	sdk "github.com/ontio/ontology-go-sdk"
	ocommon "github.com/ontio/ontology/common"
)

type Voter struct {
	allia         *sdk.OntologySdk
	voting        chan *btc.BtcProof
	wallet        *spvwallet.SPVWallet
	redeemToWatch []byte
	acct          *sdk.Account
	gasPrice      uint64
	gasLimit      uint64
	watingDB      *WatingDB
	blksToWait    uint64
}

func NewVoter(allia *sdk.OntologySdk, voting chan *btc.BtcProof, wallet *spvwallet.SPVWallet, redeem []byte,
	acct *sdk.Account, gasPrice uint64, gasLimit uint64, dbFile string, blksToWait uint64) (*Voter, error) {
	wdb, err := NewWaitingDB(dbFile)
	if err != nil {
		return nil, err
	}
	return &Voter{
		allia:         allia,
		voting:        voting,
		wallet:        wallet,
		redeemToWatch: redeem,
		acct:          acct,
		gasLimit:      gasLimit,
		gasPrice:      gasPrice,
		watingDB:      wdb,
		blksToWait:    blksToWait,
	}, nil
}

func (v *Voter) Vote() {
	log.Infof("[Voter] start voting")
	contractAddress, _ := ocommon.AddressParseFromBytes([]byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x10})

	for item := range v.voting {
		mtx, err := v.verify(item)
		switch val := err.(type) {
		case LessConfirmationError:
			go func(txid chainhash.Hash, proof *btc.BtcProof) {
				err = v.watingDB.Put(txid[:], item)
				if err != nil {
					log.Errorf("[Voter] failed to write %s into db: %v", mtx.TxHash().String(), err)
				} else if err = v.watingDB.MarkVotedTx(txid[:]); err != nil {
					log.Infof("[Voter] write %s into db and marked: %s", txid.String(), val.Error())
				} else {
					log.Errorf("[Voter] failed to mark %s: %v", txid.String(), err)
				}
			}(mtx.TxHash(), item)
			continue
		case error:
			if mtx != nil {
				log.Errorf("[Voter] failed to verify %s: %v", mtx.TxHash().String(), err)
			} else {
				log.Errorf("[Voter] : %v", err)
			}
			continue
		}

		txid := mtx.TxHash()
		log.Infof("[Voter] transaction %s passed the verify, next vote for it", txid.String())

		method := "Vote"
		param := &common.VoteParam{
			TxHash:      txid[:],
			FromChainID: BTC_CHAINID,
			Address:     v.acct.Address.ToBase58(),
		}
		txHash, err := v.allia.Native.InvokeNativeContract(v.gasPrice, v.gasLimit, v.acct, byte(0), contractAddress,
			method, []interface{}{param})
		if err != nil {
			log.Errorf("[Voter] invokeNativeContract error: %v", err)
			continue
		}

		err = v.watingDB.MarkVotedTx(txid[:])
		if err != nil {
			log.Errorf("[Voter] failed to mark tx %s: %v", err)
			continue
		}
		log.Infof("[Voter] vote yes for %s and marked. Sending transaction %s to alliance chain", txid.String(),
			txHash.ToHexString())
		//go func() { // TODO
		//	event, err := v.allia.GetSmartContractEvent(txHash.ToHexString())
		//
		//	if event.State == 0 || err != nil {
		//		log.Errorf("[Voter] voting for %s failed.(alliance transaction %s)", txid.String(), txHash.ToHexString())
		//	} else {
		//		log.Infof("[Voter] successfully")
		//	}
		//}()
	}
}

func (v *Voter) WaitingRetry() {
	log.Infof("[Voter] start retrying")
	for newh := range v.wallet.Blockchain.HeaderUpdate {
		log.Debugf("retry loop once")
		arr, keys, err := v.watingDB.GetUnderHeightAndDelte(newh - uint32(v.blksToWait) + 1)
		if err != nil {
			log.Errorf("[WaitingRetry] failed to get btcproof under height %d from db: %v", newh, err)
			continue
		} else if len(arr) == 0 {
			continue
		}

		for i, p := range arr {
			txid, _ := chainhash.NewHash(keys[i])
			log.Infof("[WaitingRetry] send txid:%s to vote", txid.String())
			v.voting <- p
		}
	}
}

func (v *Voter) verify(item *btc.BtcProof) (*wire.MsgTx, error) {
	mtx := wire.NewMsgTx(wire.TxVersion)
	err := mtx.BtcDecode(bytes.NewBuffer(item.Tx), wire.ProtocolVersion, wire.LatestEncoding)
	if err != nil {
		return nil, fmt.Errorf("verify, failed to decode transaction: %v", err)
	}
	txid := mtx.TxHash()
	if v.watingDB.CheckIfVoted(txid[:]) {
		return mtx, fmt.Errorf("verify, %s already voted or in waiting", txid.String())
	}

	bb, err := v.wallet.Blockchain.BestBlock()
	if err != nil {
		return mtx, fmt.Errorf("verify, failed to get current height from spv: %v", err)
	}
	besth := bb.Height

	if besth < item.Height || besth-item.Height < uint32(item.BlocksToWait-1) {
		return mtx, LessConfirmationError{
			Err: fmt.Errorf("verify, transaction is not confirmed, current height: %d, "+
				"input height: %d", besth, item.Height),
		}
	}

	mb := wire_bch.MsgMerkleBlock{}
	err = mb.BchDecode(bytes.NewReader(item.Proof), wire_bch.ProtocolVersion, wire_bch.LatestEncoding)
	if err != nil {
		return mtx, fmt.Errorf("verify, failed to decode proof: %v", err)
	}
	mBlock := merkleblock.NewMerkleBlockFromMsg(mb)
	merkleRootCalc := mBlock.ExtractMatches()
	if merkleRootCalc == nil || mBlock.BadTree() || len(mBlock.GetMatches()) == 0 {
		return mtx, fmt.Errorf("verify, bad merkle tree")
	}

	isExist := false
	for _, hash := range mBlock.GetMatches() {
		if bytes.Equal(hash[:], txid[:]) {
			isExist = true
			break
		}
	}
	if !isExist {
		return mtx, fmt.Errorf("verify, transaction %s not found in proof", txid.String())
	}

	err = v.checkTxOuts(mtx)
	if err != nil {
		return mtx, fmt.Errorf("verify, wrong outputs: %v", err)
	}

	err = ifCanResolve(mtx.TxOut[1], mtx.TxOut[0].Value)
	if err != nil {
		return mtx, fmt.Errorf("verify, fariled to resolve parameter: %v", err)
	}

	sh, err := v.wallet.Blockchain.GetHeaderByHeight(item.Height)
	if err != nil {
		return mtx, fmt.Errorf("verify, failed to get header from spv client: %v", err)
	}
	if !bytes.Equal(merkleRootCalc[:], sh.Header.MerkleRoot[:]) {
		return mtx, fmt.Errorf("verify, merkle root not equal")
	}

	return mtx, nil
}

func (v *Voter) checkTxOuts(tx *wire.MsgTx) error {
	if len(tx.TxOut) < 2 {
		return errors.New("checkTxOuts, number of transaction's outputs is at least greater" +
			" than 2")
	}
	if tx.TxOut[0].Value <= 0 {
		return fmt.Errorf("checkTxOuts, the value of crosschain transaction must be bigger "+
			"than 0, but value is %d", tx.TxOut[0].Value)
	}

	switch c1 := txscript.GetScriptClass(tx.TxOut[0].PkScript); c1 {
	case txscript.MultiSigTy:
		if !bytes.Equal(v.redeemToWatch, tx.TxOut[0].PkScript) {
			return fmt.Errorf("wrong script: \"%x\" is not same as our \"%x\"",
				tx.TxOut[0].PkScript, v.redeemToWatch)
		}
	case txscript.ScriptHashTy:
		addr, err := btcutil.NewAddressScriptHash(v.redeemToWatch, v.wallet.Params())
		if err != nil {
			return err
		}
		h, err := txscript.PayToAddrScript(addr)
		if err != nil {
			return err
		}
		if !bytes.Equal(h, tx.TxOut[0].PkScript) {
			return fmt.Errorf("wrong script: \"%x\" is not same as our \"%x\"", tx.TxOut[0].PkScript, h)
		}
	default:
		return errors.New("first output's pkScript is not supported")
	}

	c2 := txscript.GetScriptClass(tx.TxOut[1].PkScript)
	if c2 != txscript.NullDataTy {
		return errors.New("second output's pkScript is not NullData type")
	}

	return nil
}
