package alliance

import (
	"fmt"
	"github.com/btcsuite/btcd/btcec"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"
	"github.com/btcsuite/btcutil"
	"github.com/btcsuite/btcutil/base58"
	sdk "github.com/ontio/multi-chain-go-sdk"
	"github.com/ontio/multi-chain-go-sdk/client"
	"github.com/ontio/spvclient/config"
	"github.com/ontio/spvclient/log"
	"io/ioutil"
	"strings"
	"time"
)

type Signer struct {
	txchan chan *ToSignItem
	privk  *btcec.PrivateKey
	addr   *btcutil.AddressPubKey
	allia  *sdk.MultiChainSdk
	acct   *sdk.Account
}

func NewSigner(privkFile string, txchan chan *ToSignItem, acct *sdk.Account, allia *sdk.MultiChainSdk,
	params *chaincfg.Params) (*Signer, error) {
	data, err := ioutil.ReadFile(privkFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read btc privk: %v", err)
	}

	privk := string(data)
	privk = strings.TrimFunc(privk, func(r rune) bool {
		if r == ' ' || r == '\n' {
			return true
		}
		return false
	})
	privKey, pubk := btcec.PrivKeyFromBytes(btcec.S256(), base58.Decode(privk))
	addrPubK, err := btcutil.NewAddressPubKey(pubk.SerializeCompressed(), params)
	if err != nil {
		return nil, err
	}
	return &Signer{
		txchan: txchan,
		privk:  privKey,
		addr:   addrPubK,
		acct:   acct,
		allia:  allia,
	}, nil
}

func (signer *Signer) Signing() {
	log.Infof("[Signer] start signing")

	for {
		select {
		case item := <-signer.txchan:
			txHash := item.Mtx.TxHash()
			sigs, err := signer.getSigs(item.Mtx, item.Redeem)
			if err != nil {
				log.Errorf("[Signer] failed to sign (unsigned tx hash %s), not supposed to happen: "+
					"%v", txHash.String(), err)
				continue
			}
			txid, err := signer.allia.Native.Ccm.BtcMultiSign(txHash[:], signer.addr.EncodeAddress(), sigs, signer.acct)
			if err != nil {
				switch err.(type) {
				case client.PostErr:
					go func() {
						signer.txchan <- item
					}()
					log.Errorf("[Signer] post err and would retry after %d sec: %v", config.SleepTime, err)
					wait(time.Second * config.SleepTime)
				default:
					log.Errorf("[Signer] failed to invoke alliance: %v", err)
				}
				continue
			}
			log.Infof("[Signer] signed for btc tx %s and send tx %s to alliance", txHash.String(), txid.ToHexString())
		}
	}
}

func (signer *Signer) getSigs(tx *wire.MsgTx, redeem []byte) ([][]byte, error) {
	sigs := make([][]byte, 0)
	for i, _ := range tx.TxIn {
		sig, err := txscript.RawTxInSignature(tx, i, redeem, txscript.SigHashAll, signer.privk)
		if err != nil {
			return nil, fmt.Errorf("Failed to sign tx's No.%d input: %v", i, err)
		}
		sigs = append(sigs, sig)
	}

	return sigs, nil
}
