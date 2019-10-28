package service

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"github.com/ontio/spvwallet"
	"github.com/ontio/spvwallet/log"
	"github.com/ontio/spvwallet/rest/http/common"
	"github.com/ontio/spvwallet/rest/http/restful"
	"github.com/ontio/spvwallet/rest/utils"
	"github.com/btcsuite/btcd/wire"
	"time"
)

type Service struct {
	wallet *spvwallet.SPVWallet
	cfg    *spvwallet.RestConfig
}

func NewService(wallet *spvwallet.SPVWallet, cfg *spvwallet.RestConfig) *Service {
	return &Service{
		wallet: wallet,
		cfg:    cfg,
	}
}

func (serv *Service) QueryHeaderByHeight(params map[string]interface{}) map[string]interface{} {
	req := &common.QueryHeaderByHeightReq{}
	resp := &common.Response{}
	failedRes := &common.QueryHeaderByHeightResp{
		Header: "",
	}

	err := utils.ParseParams(req, params)
	if err != nil {
		resp.Error = restful.INVALID_PARAMS
		resp.Desc = err.Error()
		resp.Result = failedRes
		log.Errorf("QueryHeaderByHeight: decode params failed, err: %s", err)
	} else {
		header, err := serv.wallet.Blockchain.GetHeaderByHeight(req.Height)
		if err != nil {
			resp.Error = restful.INTERNAL_ERROR
			resp.Desc = err.Error()
			resp.Result = failedRes
			log.Errorf("QueryHeaderByHeight: v%", err)
		} else {
			var buf bytes.Buffer
			err = header.Header.BtcEncode(&buf, wire.ProtocolVersion, wire.LatestEncoding)
			resp.Error = restful.SUCCESS
			resp.Result = &common.QueryHeaderByHeightResp{
				Header: hex.EncodeToString(buf.Bytes()),
			}
		}
	}

	m, err := utils.RefactorResp(resp, resp.Error)
	if err != nil {
		log.Errorf("QueryHeaderByHeight: failed, err: %s", err)
	} else {
		log.Info("QueryHeaderByHeight: resp success")
	}
	return m
}

func (serv *Service) GetCurrentHeight(params map[string]interface{}) map[string]interface{} {
	resp := &common.Response{}
	sh, err := serv.wallet.Blockchain.BestBlock()
	if err != nil {
		resp.Error = restful.INTERNAL_ERROR
		resp.Desc = err.Error()
		resp.Result = &common.GetCurrentHeightResp{
			Height: 0,
		}
		log.Errorf("GetCurrentHeight, failed to get best block: %v", err)
	} else {
		resp.Error = restful.SUCCESS
		resp.Result = &common.GetCurrentHeightResp{
			Height: sh.Height,
		}
	}

	m, err := utils.RefactorResp(resp, resp.Error)
	if err != nil {
		log.Errorf("GetCurrentHeight: failed, err: %s", err)
	} else {
		log.Info("GetCurrentHeight: resp success")
	}
	return m
}

func (serv *Service) Rollback(params map[string]interface{}) map[string]interface{} {
	req := &common.RollbackReq{}
	resp := &common.Response{}

	prevh, _ := serv.wallet.ChainTip()
	err := utils.ParseParams(req, params)
	if err != nil {
		resp.Error = restful.INVALID_PARAMS
		resp.Desc = err.Error()
		log.Errorf("Rollback: decode params failed, err: %s", err)
	} else {
		t, err := time.Parse("2006-01-02 15:04:05", req.Time)
		if err != nil {
			resp.Error = restful.INTERNAL_ERROR
			resp.Desc = fmt.Sprintf("failed to parse time %s: %v", req.Time, err)
			log.Errorf("Rollback: failed to parse time %s: %v", req.Time, err)
		} else {
			serv.wallet.ReSyncBlockchain(t)
			resp.Error = restful.SUCCESS
			resp.Result = nil
		}
	}

	m, err := utils.RefactorResp(resp, resp.Error)
	if err != nil {
		log.Errorf("Rollback: failed, err: %s", err)
	} else {
		h, _ := serv.wallet.ChainTip()
		log.Infof("Rollback: resp success, roll back to height %d from height %d", h, prevh)
	}
	return m
}

func (serv *Service) BroadcastTx(params map[string]interface{}) map[string]interface{} {
	req := &common.BroadcastReq{}
	resp := &common.Response{}

	var txid string
	err := utils.ParseParams(req, params)
	if err != nil {
		resp.Error = restful.INVALID_PARAMS
		resp.Desc = err.Error()
		log.Errorf("BroadcastTx: decode params failed, err: %s", err)
	} else {
		btx, err := hex.DecodeString(req.Tx)
		if err != nil {
			resp.Error = restful.INTERNAL_ERROR
			resp.Desc = err.Error()
			log.Errorf("BroadcastTx: decode hex failed: %v", err)
		} else {
			mtx := wire.NewMsgTx(wire.TxVersion)
			if err := mtx.BtcDecode(bytes.NewBuffer(btx), wire.ProtocolVersion, wire.LatestEncoding); err != nil {
				resp.Error = restful.INTERNAL_ERROR
				resp.Desc = err.Error()
				log.Errorf("BroadcastTx: decode msgtx failed: %v", err)
			} else {
				txid = mtx.TxHash().String()
				if err := serv.wallet.Broadcast(mtx); err != nil {
					resp.Error = restful.INTERNAL_ERROR
					resp.Desc = err.Error()
					log.Errorf("BroadcastTx: broadcast msgtx failed: %v", err)
				}
			}
		}
	}

	m, err := utils.RefactorResp(resp, resp.Error)
	if err != nil {
		log.Errorf("BroadcastTx: failed, err: %s", err)
	} else {
		log.Infof("BroadcastTx: resp success, txid is %s", txid)
	}
	return m
}
