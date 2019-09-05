package alliance

import (
	"encoding/hex"
	"github.com/Zou-XueYan/spvwallet/log"
	"github.com/btcsuite/btcd/wire"
	mc "github.com/ontio/multi-chain/common"
	"github.com/ontio/multi-chain/smartcontract/service/native/cross_chain_manager/btc"
	sdk "github.com/ontio/ontology-go-sdk"
	"github.com/ontio/ontology-go-sdk/common"
	"time"
)

type ObConfig struct {
	FirstN       int
	LoopWaitTime int64
	WatchingKey  string
}

type Observer struct {
	voting chan *btc.BtcProof
	allia  *sdk.OntologySdk
	conf   *ObConfig
}

func NewObserver(allia *sdk.OntologySdk, conf *ObConfig, voting chan *btc.BtcProof) *Observer {
	return &Observer{
		voting: voting,
		conf:   conf,
		allia:  allia,
	}
}

func (ob *Observer) Listen() {
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
	log.Infof("[Observer] first to start Listen(), check %d blocks from top %d", num, top)
	for num > 0 && h+1 > 0 {
		events, err := ob.allia.GetSmartContractEventByBlock(h)
		if err != nil {
			log.Errorf("[Observer] GetSmartContractEventByBlock failed, retry after 10 sec: %v", err)
			time.Sleep(time.Second * 10)
			continue
		}
		count += ob.checkEvents(events, h)
		num--
		h--
	}
	log.Infof("[Observer] total %d transactions captured from %d blocks", count, ob.conf.FirstN)

	log.Infof("[Observer] next, check once %d seconds", ob.conf.LoopWaitTime)
	for {
		time.Sleep(time.Second * time.Duration(ob.conf.LoopWaitTime))
		count = 0
		newTop, err := ob.allia.GetCurrentBlockHeight()
		if err != nil {
			log.Errorf("[Observer] failed to get current height, retry after 10 sec: %v", err)
			time.Sleep(time.Second * 10)
			continue
		}

		num := int64(newTop - top)
		if num == 0 {
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
			count += ob.checkEvents(events, h)
			num--
			h--
		}
		if count > 0 {
			log.Infof("[Observer] total %d transactions captured this time", count)
		}
		top = newTop
	}
}

func (ob *Observer) checkEvents(events []*common.SmartContactEvent, h uint32) int {
	count := 0
	for _, e := range events {
		for _, n := range e.Notify {
			states, ok := n.States.([]interface{})
			if !ok {
				continue
			}

			name, ok := states[0].(string)
			if ok && name == ob.conf.WatchingKey {
				btcProofBytes, err := hex.DecodeString(states[1].(string))
				if err != nil {
					continue
				}

				btcProof := btc.BtcProof{}
				err = btcProof.Deserialization(mc.NewZeroCopySource(btcProofBytes))
				if err != nil {
					continue
				}

				mtx := wire.NewMsgTx(wire.TxVersion)
				txid := mtx.TxHash()

				ob.voting <- &btcProof
				count++
				log.Infof("[Observer] captured: height is %d, txid is %s", h, txid.String())
			}
		}
	}

	return count
}
