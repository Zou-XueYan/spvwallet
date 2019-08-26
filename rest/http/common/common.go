package common

import (
	"github.com/btcsuite/btcd/btcjson"
)

const (
	QUERYHEADERBYHEIGHT = "/api/v1/queryheaderbyheight"
	QUERYUTXOS          = "/api/v1/queryutxos"
	GETCURRENTHEIGHT    = "/api/v1/getcurrentheight"
	CHANGEADDRESS       = "/api/v1/changeaddress"
	GETALLADDRESS       = "/api/v1/getalladdress"
	UNLOCKUTXO          = "/api/v1/unlockutxo"
	GETFEEPERBYTE       = "/api/v1/getfeeperbyte"
	GETALLUTXOS         = "/api/v1/getallutxos"
	BROADCASTTX         = "/api/v1/broadcasttx"
	ROLLBACK            = "/api/v1/rollback"
	DELUTXO = "/api/v1/delutxo"
)

const (
	ACTION_QUERYHEADERBYHEIGHT = "queryheaderbyheight"
	ACTION_QUERYUTXOS          = "queryutxos"
	ACTION_GETCURRENTHEIGHT    = "getcurrentheight"
	ACTION_CHANGEADDRESS       = "changeaddress"
	ACTION_GETALLADDRESS       = "getalladdress"
	ACTION_UNLOCKUTXO          = "unlockutxo"
	ACTION_GETFEEPERBYTE       = "getfeeperbyte"
	ACTION_GETALLUTXOS         = "getallutxos"
	ACTION_BROADCASTTX         = "broadcasttx"
	ACTION_ROLLBACK            = "rollback"
	ACTION_DELUTXO = "delutxo"
)

type Response struct {
	Action string      `json:"action"`
	Desc   string      `json:"desc"`
	Error  uint32      `json:"error"`
	Result interface{} `json:"result"`
}

type QueryHeaderByHeightReq struct {
	Height uint32 `json:"height"`
}

type QueryHeaderByHeightResp struct {
	Header string `json:"header"`
}

type QueryUtxosReq struct {
	Addr      string `json:"addr"`
	Amount    int64  `json:"amount"`
	Fee       int64  `json:"fee"`
	IsPreExec bool   `json:"is_pre_exec"`
}

type QueryUtxosResp struct {
	Inputs []btcjson.TransactionInput `json:"inputs"`
	Sum    int64                      `json:"sum"`
}

type GetCurrentHeightResp struct {
	Height uint32 `json:"height"`
}

type ChangeAddressReq struct {
	Aciton string `json:"aciton"`
	Addr   string `json:"addr"`
}

type GetAllAddressResp struct {
	Addresses []string `json:"addresses"`
}

type UnlockUtxoReq struct {
	Hash  string `json:"hash"`
	Index uint32 `json:"index"`
}

type GetFeePerByteReq struct {
	Level int `json:"level"`
}

type GetFeePerByteResp struct {
	Feepb uint64 `json:"feepb"`
}

type UtxoInfo struct {
	Outpoint string `json:"outpoint"`
	Val      int64  `json:"val"`
	IsLock   bool   `json:"is_lock"`
	Height   int32  `json:"height"`
	Script   string `json:"script"`
}

type GetAllUtxosResp struct {
	Infos []UtxoInfo `json:"infos"`
}

type BroadcastTxReq struct {
	RawTx string `json:"raw_tx"`
}

type RollbackReq struct {
	Time string `json:"time"`
}

type DelUtxoReq struct {
	Op btcjson.OutPoint `json:"op"`
}
