package alliance

import (
	"bytes"
	"encoding/hex"
	"github.com/btcsuite/btcd/wire"
	sdk "github.com/ontio/multi-chain-go-sdk"
	"github.com/ontio/multi-chain-go-sdk/client"
	"github.com/ontio/multi-chain-go-sdk/common"
	mc "github.com/ontio/multi-chain/common"
	"github.com/ontio/multi-chain/native/service/cross_chain_manager/btc"
	"github.com/ontio/spvwallet/config"
	"github.com/ontio/spvwallet/log"
	"time"
)

type Observer struct {
	voting            chan *btc.BtcProof
	txchan            chan *ToSignItem
	allia             *sdk.MultiChainSdk
	loopWaitTime      int64
	watchingKey       string
	watchingMakeTxKey string
	netType           string
}

func NewObserver(allia *sdk.MultiChainSdk, voting chan *btc.BtcProof, txchan chan *ToSignItem, loopWaitTime int64,
	watchingKey, watchingMakeTxKey, netType string) *Observer {
	return &Observer{
		voting:            voting,
		allia:             allia,
		txchan:            txchan,
		watchingMakeTxKey: watchingMakeTxKey,
		watchingKey:       watchingKey,
		loopWaitTime:      loopWaitTime,
		netType:           netType,
	}
}

func (ob *Observer) Listen() {
	log.Info("starting observing")
	top := alliaCheckPoints[ob.netType].Height

	log.Infof("[AllianceObserver] get start height %d from checkpoint, check once %d seconds", top, ob.loopWaitTime)
	tick := time.NewTicker(time.Second * time.Duration(ob.loopWaitTime))
	defer tick.Stop()

	for {
		select {
		case <-tick.C:
			log.Debugf("observe once")
			toVote, toSign := 0, 0
			newTop, err := ob.allia.GetCurrentBlockHeight()
			if err != nil {
				log.Errorf("[Observer] failed to get current height, retry after 10 sec: %v", err)
				<-time.Tick(time.Second * config.SleepTime)
				continue
			}

			if newTop-top <= 0 {
				continue
			}
			h := top + 1
			for h <= newTop {
				events, err := ob.allia.GetSmartContractEventByBlock(h)
				if err != nil {
					switch err.(type) {
					case client.PostErr:
						log.Errorf("[Observer] GetSmartContractEventByBlock failed, retry after 10 sec: %v", err)
					default:
						log.Errorf("[Observer] not supposed to happen: %v", err)
					}
					<-time.Tick(time.Second * config.SleepTime)
					continue
				}
				voteCnt, signCnt := ob.checkEvents(events, h)
				toVote += voteCnt
				toSign += signCnt
				h++
			}
			if toVote > 0 {
				log.Infof("[Observer] btc tx to vote: total %d transactions captured this time", toVote)
			}
			if toSign > 0 {
				log.Infof("[Observer] btc tx to sig: total %d transactions captured this time", toSign)
			}
			top = newTop
		}
	}
}

func (ob *Observer) checkEvents(events []*common.SmartContactEvent, h uint32) (int, int) {
	toVote, toSign := 0, 0
	for _, e := range events {
		for _, n := range e.Notify {
			states, ok := n.States.([]interface{})
			if !ok {
				continue
			}

			name, ok := states[0].(string)
			if ok && name == ob.watchingKey {
				txid := states[1].(string)
				btcProofBytes, err := hex.DecodeString(states[2].(string))
				if err != nil {
					log.Errorf("[Observer] wrong hex from chain, not supposed to happen: %v", err)
					continue
				}

				btcProof := btc.BtcProof{}
				err = btcProof.Deserialization(mc.NewZeroCopySource(btcProofBytes))
				if err != nil {
					log.Errorf("[Observer] failed deserialization for btc proof from chain, not supposed to happen"+
						": %v", err)
					continue
				}

				ob.voting <- &btcProof
				toVote++
				log.Infof("[Observer] captured %s proof when height is %d", txid, h)
			} else if ok && name == ob.watchingMakeTxKey {
				txb, err := hex.DecodeString(states[1].(string))
				if err != nil {
					log.Errorf("[Observer] wrong hex-string of tx from chain, not supposed to happen: %v", err)
					continue
				}
				mtx := wire.NewMsgTx(wire.TxVersion)
				err = mtx.BtcDecode(bytes.NewBuffer(txb), wire.ProtocolVersion, wire.LatestEncoding)
				if err != nil {
					log.Errorf("[Observer] failed to decode btc transaction from chain, not supposed to happen: "+
						"%v", err)
					continue
				}

				redeem, err := hex.DecodeString(states[2].(string))
				if err != nil {
					log.Errorf("[Observer] failed to decode hex-string of tx, not supposed to happen: %v", err)
					continue
				}
				ob.txchan <- &ToSignItem{
					Mtx:    mtx,
					Redeem: redeem,
				}
				toSign++
				log.Infof("[Observer] captured one tx when height is %d", h)
			}
		}
	}

	return toVote, toSign
}
