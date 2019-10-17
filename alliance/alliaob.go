package alliance

import (
	"bytes"
	"encoding/hex"
	"github.com/Zou-XueYan/spvwallet/log"
	"github.com/btcsuite/btcd/wire"
	sdk "github.com/ontio/multi-chain-go-sdk"
	"github.com/ontio/multi-chain-go-sdk/common"
	mc "github.com/ontio/multi-chain/common"
	"github.com/ontio/multi-chain/native/service/cross_chain_manager/btc"
	"time"
)

type ObConfig struct {
	FirstN            int
	LoopWaitTime      int64
	WatchingKey       string
	WatchingMakeTxKey string
}

type Observer struct {
	voting chan *btc.BtcProof
	txchan chan *ToSignItem
	allia  *sdk.MultiChainSdk
	conf   *ObConfig
}

func NewObserver(allia *sdk.MultiChainSdk, conf *ObConfig, voting chan *btc.BtcProof, txchan chan *ToSignItem) *Observer {
	return &Observer{
		voting: voting,
		conf:   conf,
		allia:  allia,
		txchan: txchan,
	}
}

func (ob *Observer) Listen() {
	log.Info("starting observing")
START:
	top, err := ob.allia.GetCurrentBlockHeight()
	if err != nil {
		log.Errorf("[Observer] failed to get current height: %v", err)
		time.Sleep(time.Second * 30)
		goto START
	}

	num := ob.conf.FirstN
	h := uint32(top)
	count := 0
	countTx := 0
	log.Infof("[Observer] first to start Listen(), check %d blocks from top %d", num, top)
	for num > 0 && h+1 > 0 {
		events, err := ob.allia.GetSmartContractEventByBlock(h)
		if err != nil {
			log.Errorf("[Observer] GetSmartContractEventByBlock failed, retry after 10 sec: %v", err)
			time.Sleep(time.Second * 10)
			continue
		}
		cnt, cntTx := ob.checkEvents(events, h)
		count += cnt
		countTx += cntTx
		num--
		h--
	}
	log.Infof("[Observer] btc proof: total %d transactions captured from %d blocks", count, ob.conf.FirstN)
	log.Infof("[Observer] btc tx: total %d transactions captured from %d blocks", countTx, ob.conf.FirstN)

	log.Infof("[Observer] next, check once %d seconds", ob.conf.LoopWaitTime)
	tick := time.NewTicker(time.Second * time.Duration(ob.conf.LoopWaitTime))
	defer tick.Stop()

	for {
		select {
		case <-tick.C:
			log.Debugf("observe once")
			count = 0
			countTx = 0
			newTop, err := ob.allia.GetCurrentBlockHeight()
			if err != nil {
				log.Errorf("[Observer] failed to get current height, retry after 10 sec: %v", err)
				time.Sleep(time.Second * 10)
				continue
			}

			num := int64(newTop - top)
			if num <= 0 {
				continue
			}

			h := newTop
			for num > 0 {
				events, err := ob.allia.GetSmartContractEventByBlock(h)
				if err != nil {
					log.Errorf("[Observer] GetSmartContractEventByBlock failed, retry after 10 sec: %v", err)
					time.Sleep(time.Second * 10)
					continue
				}
				cnt, cntTx := ob.checkEvents(events, h)
				count += cnt
				countTx += cntTx
				num--
				h--
			}
			if count > 0 {
				log.Infof("[Observer] btc proof: total %d transactions captured this time", count)
			}
			if countTx > 0 {
				log.Infof("[Observer] btc tx: total %d transactions captured this time", countTx)
			}
			top = newTop
			//case <-ob.quit:
			//	log.Info("stopping observing alliance network")
			//	return
		}
	}
}

func (ob *Observer) checkEvents(events []*common.SmartContactEvent, h uint32) (int, int) {
	count := 0
	countTx := 0
	for _, e := range events {
		for _, n := range e.Notify {
			states, ok := n.States.([]interface{})
			if !ok {
				continue
			}

			name, ok := states[0].(string)
			if ok && name == ob.conf.WatchingKey {
				txid := states[1].(string)
				btcProofBytes, err := hex.DecodeString(states[2].(string))
				if err != nil {
					log.Errorf("[Observer] failed to decode hex: %v", err)
					continue
				}

				btcProof := btc.BtcProof{}
				err = btcProof.Deserialization(mc.NewZeroCopySource(btcProofBytes))
				if err != nil {
					log.Errorf("[Observer] failed to deserialization: %v", err)
					continue
				}

				ob.voting <- &btcProof
				count++
				log.Infof("[Observer] captured %s proof when height is %d", txid, h)
			} else if ok && name == ob.conf.WatchingMakeTxKey {
				txb, err := hex.DecodeString(states[1].(string))
				if err != nil {
					log.Errorf("[Observer] failed to decode hex-string of tx: %v", err)
					continue
				}
				mtx := wire.NewMsgTx(wire.TxVersion)
				err = mtx.BtcDecode(bytes.NewBuffer(txb), wire.ProtocolVersion, wire.LatestEncoding)
				if err != nil {
					log.Errorf("[Observer] failed to decode btc transaction: %v", err)
					continue
				}

				redeem, err := hex.DecodeString(states[2].(string))
				if err != nil {
					log.Errorf("[Observer] failed to decode hex-string of tx: %v", err)
					continue
				}
				ob.txchan <- &ToSignItem{
					Mtx:    mtx,
					Redeem: redeem,
				}
				countTx++
				log.Infof("[Observer] captured one tx when height is %d", h)
			}
		}
	}

	return count, countTx
}
