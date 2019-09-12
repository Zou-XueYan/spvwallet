package service

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"github.com/Zou-XueYan/spvwallet"
	"github.com/Zou-XueYan/spvwallet/interface"
	"github.com/Zou-XueYan/spvwallet/log"
	"github.com/Zou-XueYan/spvwallet/rest/config"
	"github.com/Zou-XueYan/spvwallet/rest/http/common"
	"github.com/Zou-XueYan/spvwallet/rest/http/restful"
	"github.com/Zou-XueYan/spvwallet/rest/utils"
	"github.com/btcsuite/btcd/btcjson"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"
	"github.com/btcsuite/btcutil"
	"github.com/btcsuite/btcutil/coinset"
	"time"
)

type Coin struct {
	TxHash       *chainhash.Hash
	TxIndex      uint32
	TxValue      btcutil.Amount
	TxNumConfs   int64
	ScriptPubKey []byte
}

func (c *Coin) Hash() *chainhash.Hash { return c.TxHash }
func (c *Coin) Index() uint32         { return c.TxIndex }
func (c *Coin) Value() btcutil.Amount { return c.TxValue }
func (c *Coin) PkScript() []byte      { return c.ScriptPubKey }
func (c *Coin) NumConfs() int64       { return c.TxNumConfs }
func (c *Coin) ValueAge() int64       { return int64(c.TxValue) * c.TxNumConfs }

type Service struct {
	wallet *spvwallet.SPVWallet
	cfg    *config.Config
}

func NewService(wallet *spvwallet.SPVWallet, cfg *config.Config) *Service {
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

func (serv *Service) QueryUtxos(params map[string]interface{}) map[string]interface{} {
	req := &common.QueryUtxosReq{}
	resp := &common.Response{}
	failedRes := &common.QueryUtxosResp{
		Inputs: nil,
		Sum:    -1,
	}
	err := utils.ParseParams(req, params)
	if err != nil {
		resp.Error = restful.INVALID_PARAMS
		resp.Desc = err.Error()
		resp.Result = failedRes
		log.Errorf("QueryUtxos: decode params failed, err: %s", err)
	} else {
		allUtxos, err := serv.wallet.GetTxStore().Utxos().GetAll()
		if err != nil {
			resp.Error = restful.INTERNAL_ERROR
			resp.Desc = err.Error()
			resp.Result = failedRes
			log.Errorf("QueryUtxos: failed to get all utxos: %v", err)
		} else if len(allUtxos) == 0 || allUtxos == nil {
			resp.Error = restful.INTERNAL_ERROR
			resp.Desc = "no utxo to use"
			resp.Result = failedRes
			log.Errorf("QueryUtxos: no utxo to use")
		} else {
			chosed, sum, err := serv.getRequestedUtxos(req.Addr, req.Amount, req.Fee, allUtxos)
			if err != nil {
				resp.Error = restful.INTERNAL_ERROR
				resp.Desc = fmt.Sprintf("Failed to find utxo: %v", err)
				resp.Result = failedRes
				log.Errorf("QueryUtxos: failed to find utxos: %v", err)
			} else if len(chosed) == 0 || sum <= 0 {
				resp.Error = restful.INTERNAL_ERROR
				resp.Desc = fmt.Sprintf("No utxo meet the requirements")
				resp.Result = failedRes
				log.Errorf("QueryUtxos: No utxo meet the requirements")
			} else {
				infos := make([]btcjson.TransactionInput, 0)
				for i, u := range chosed {
					if !req.IsPreExec {
						err = serv.wallet.GetTxStore().Utxos().Lock(u)
						log.Infof("QueryUtxos: locking utxo %s", u.Op.String())
					}
					infos = append(infos, btcjson.TransactionInput{u.Op.Hash.String(), u.Op.Index})
					if err != nil {
						resp.Error = restful.INTERNAL_ERROR
						resp.Desc = err.Error()
						resp.Result = failedRes
						log.Errorf("QueryUtxos: failed to lock utxo %s: %v", u.Op.String(), err)

						for _, uu := range chosed[:i+1] {
							serv.wallet.GetTxStore().Utxos().Unlock(uu.Op)
						}
						break
					}
					log.Infof("QueryUtxos: sending utxo %s", u.Op.String())
				}
				if err == nil {
					resp.Error = restful.SUCCESS
					resp.Result = &common.QueryUtxosResp{
						Inputs: infos,
						Sum:    sum,
					}
				}
			}
		}
	}

	m, err := utils.RefactorResp(resp, resp.Error)
	if err != nil {
		log.Errorf("QueryUtxos: failed, err: %s", err)
	} else {
		log.Info("QueryUtxos: resp success")
	}
	return m
}

func (serv *Service) getRequestedUtxos(addr string, amount int64, fee int64, utxos []wallet.Utxo) ([]wallet.Utxo, int64, error) {
	used, confs, err := serv.gatherUtxos(addr, utxos)
	if err != nil {
		return nil, 0, fmt.Errorf("Failed to gather coins: %v", err)
	}

	coins := make([]coinset.Coin, 0)
	for key := range used {
		u, _ := used[key]
		confirmations, ok := confs[key]
		if !ok {
			return nil, 0, fmt.Errorf("Failed to get %s from map", key)
		}
		c := coinset.Coin(&Coin{
			TxHash:       &u.Op.Hash,
			TxIndex:      u.Op.Index,
			TxValue:      btcutil.Amount(u.Value),
			TxNumConfs:   int64(confirmations),
			ScriptPubKey: u.ScriptPubkey,
		})
		coins = append(coins, c)
	}

	coinSelector := coinset.MaxValueAgeCoinSelector{
		MaxInputs:       10000,
		MinChangeAmount: btcutil.Amount(0),
	}
	coinsToUse, err := coinSelector.CoinSelect(btcutil.Amount(amount+fee), coins)
	if err != nil {
		return nil, 0, fmt.Errorf("Failed to select coin: %v", err)
	}
	if len(coinsToUse.Coins()) == 0 {
		return nil, 0, fmt.Errorf("Utxo not enough to pay %d satoshi", amount+fee)
	}

	sum := int64(0)
	ret := make([]wallet.Utxo, 0)
	for _, c := range coinsToUse.Coins() {
		key := wire.OutPoint{*c.Hash(), c.Index()}.String()
		val, _ := used[key]
		ret = append(ret, val)
		sum += int64(c.Value())
		log.Infof("getRequestedUtxos, utxo to return is %s", key)
	}

	return ret, sum, nil
}

func (serv *Service) gatherUtxos(addr string, utxos []wallet.Utxo) (map[string]wallet.Utxo, map[string]int32, error) {
	height, _ := serv.wallet.ChainTip()
	address, err := btcutil.DecodeAddress(addr, serv.wallet.Params())
	if err != nil {
		return nil, nil, fmt.Errorf("Failed to decode address from string: %v", err)
	}
	script, err := serv.wallet.AddressToScript(address)
	if err != nil {
		return nil, nil, fmt.Errorf("Failed to transfer address to script: %v", err)
	}
	confs := make(map[string]int32)
	usedUtxos := make(map[string]wallet.Utxo)
	for _, u := range utxos {
		if !bytes.Equal(u.ScriptPubkey, script) || u.Lock {
			continue
		}
		var confirmations int32
		if u.AtHeight > 0 {
			confirmations = int32(height) - u.AtHeight + 1
		}
		if confirmations < 6 {
			continue
		}

		confs[u.Op.String()] = confirmations
		usedUtxos[u.Op.String()] = u
	}
	return usedUtxos, confs, nil
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

func (serv *Service) ChangeAddress(params map[string]interface{}) map[string]interface{} {
	req := &common.ChangeAddressReq{}
	resp := &common.Response{}

	err1 := utils.ParseParams(req, params)
	addr, err2 := btcutil.DecodeAddress(req.Addr, serv.wallet.Params())
	if err1 != nil {
		resp.Error = restful.INVALID_PARAMS
		resp.Desc = err1.Error()
		log.Errorf("ChangeAddress: decode params failed, err: %s", err1)
	} else if err2 != nil {
		resp.Error = restful.INTERNAL_ERROR
		resp.Desc = err2.Error()
		log.Errorf("ChangeAddress: failed to decode address in string: %v", err2)
	} else {
		switch req.Aciton {
		case "add":
			err := serv.wallet.AddWatchedAddress(addr)
			if err != nil {
				resp.Error = restful.INTERNAL_ERROR
				resp.Desc = err.Error()
				log.Errorf("ChangeAddress: failed to add watched script: %v", err)
			} else {
				resp.Error = restful.SUCCESS
			}
		case "delete":
			err := serv.wallet.DeleteWatchedAddr(addr)
			if err != nil {
				resp.Error = restful.INTERNAL_ERROR
				resp.Desc = err.Error()
				log.Errorf("ChangeAddress: failed to delete watched script: %v", err)
			} else {
				resp.Error = restful.SUCCESS
			}
		default:
			resp.Error = restful.INTERNAL_ERROR
			resp.Desc = "action not found"
			log.Errorf("ChangeAddress: action not found: %s", req.Aciton)
		}
	}

	m, err := utils.RefactorResp(resp, resp.Error)
	if err != nil {
		log.Errorf("ChangeAddress: failed, err: %v", err)
	} else {
		log.Infof("ChangeAddress: resp success, %s %s", req.Aciton, req.Addr)
	}
	return m
}

func (serv *Service) GetAllAddress(params map[string]interface{}) map[string]interface{} {
	resp := &common.Response{}
	ss, err := serv.wallet.GetTxStore().WatchedScripts().GetAll()
	if err != nil || len(ss) == 0 {
		resp.Error = restful.INTERNAL_ERROR
		resp.Desc = err.Error()
		resp.Result = &common.GetAllAddressResp{
			Addresses: nil,
		}
		if err != nil {
			log.Errorf("GetAllAddress, failed to get all address: %v", err)
		} else {
			log.Errorf("GetAllAddress, no watched address")
		}
	} else {
		addrs := make([]string, 0)
		for _, s := range ss {
			addr, _ := serv.wallet.ScriptToAddress(s)
			addrs = append(addrs, addr.EncodeAddress())
		}
		resp.Error = restful.SUCCESS
		resp.Result = &common.GetAllAddressResp{
			Addresses: addrs,
		}
	}

	m, err := utils.RefactorResp(resp, resp.Error)
	if err != nil {
		log.Errorf("GetAllAddress: failed, err: %s", err)
	} else {
		log.Info("GetAllAddress: resp success")
	}
	return m
}

func (serv *Service) UnlockUtxo(params map[string]interface{}) map[string]interface{} {
	req := &common.UnlockUtxoReq{}
	resp := &common.Response{}

	ret := ""
	err := utils.ParseParams(req, params)
	if err != nil {
		resp.Error = restful.INVALID_PARAMS
		resp.Desc = err.Error()
		log.Errorf("UnlockUtxo: decode params failed, err: %s", err)
	} else {
		hash, err := chainhash.NewHashFromStr(req.Hash)
		if err != nil {
			resp.Error = restful.INTERNAL_ERROR
			resp.Desc = err.Error()
			log.Errorf("UnlockUtxo: failed to decode hash: %v", err)
		} else {
			out := wire.OutPoint{
				Hash:  *hash,
				Index: req.Index,
			}
			err = serv.wallet.GetTxStore().Utxos().Unlock(out)
			if err != nil {
				resp.Error = restful.INTERNAL_ERROR
				resp.Desc = err.Error()
				log.Errorf("UnlockUtxo: failed to unlock: %v", err)
			} else {
				resp.Error = restful.SUCCESS
				ret = out.String()
			}
		}
	}

	m, err := utils.RefactorResp(resp, resp.Error)
	if err != nil {
		log.Errorf("UnlockUtxo: failed, err: %s", err)
	} else {
		log.Infof("UnlockUtxo: resp success, unlock utxo %s", ret)
	}
	return m
}

func (serv *Service) GetFeePerByte(params map[string]interface{}) map[string]interface{} {
	req := &common.GetFeePerByteReq{}
	resp := &common.Response{}

	rate := serv.wallet.GetFeePerByte(wallet.FeeLevel(req.Level))
	err := utils.ParseParams(req, params)
	if err != nil {
		resp.Error = restful.INVALID_PARAMS
		resp.Desc = err.Error()
		log.Errorf("GetFeePerByte: decode params failed, err: %s", err)
	} else {
		resp.Error = restful.SUCCESS
		resp.Result = &common.GetFeePerByteResp{
			Feepb: rate,
		}
	}

	m, err := utils.RefactorResp(resp, resp.Error)
	if err != nil {
		log.Errorf("UnlockUtxo: failed, err: %s", err)
	} else {
		log.Infof("UnlockUtxo: resp success, rate(satoshi/byte)=%d", rate)
	}
	return m
}

func (serv *Service) GetAllUtxos(params map[string]interface{}) map[string]interface{} {
	resp := &common.Response{}
	utxos, err := serv.wallet.GetTxStore().Utxos().GetAll()
	if err != nil {
		resp.Error = restful.INTERNAL_ERROR
		resp.Desc = err.Error()
		resp.Result = &common.GetAllAddressResp{
			Addresses: nil,
		}
		log.Errorf("Failed to get utxos: %v", err)
	} else if len(utxos) == 0 {
		resp.Error = restful.INTERNAL_ERROR
		resp.Desc = err.Error()
		resp.Result = &common.GetAllAddressResp{
			Addresses: nil,
		}
		log.Errorf("no utxo in db")
	} else {
		ret := make([]common.UtxoInfo, 0)
		for _, u := range utxos {
			s, _ := txscript.DisasmString(u.ScriptPubkey)
			ret = append(ret, common.UtxoInfo{
				Outpoint: u.Op.String(),
				Val:      u.Value,
				IsLock:   u.Lock,
				Height:   u.AtHeight,
				Script:   s,
			})

			log.Infof("GetAllUtxos: sending utxo %s", u.Op.String())
		}
		resp.Error = restful.SUCCESS
		resp.Result = &common.GetAllUtxosResp{
			Infos: ret,
		}
	}

	m, err := utils.RefactorResp(resp, resp.Error)
	if err != nil {
		log.Errorf("GetAllUtxos: failed, err: %s", err)
	} else {
		log.Info("GetAllUtxos: resp success")
	}
	return m
}

func (serv *Service) BroadcastTx(params map[string]interface{}) map[string]interface{} {
	req := &common.BroadcastTxReq{}
	resp := &common.Response{}

	var txid string
	err := utils.ParseParams(req, params)
	if err != nil {
		resp.Error = restful.INVALID_PARAMS
		resp.Desc = err.Error()
		log.Errorf("BroadcastTx: decode params failed, err: %s", err)
	} else {
		mtx := wire.NewMsgTx(wire.TxVersion)
		rawtx, err := hex.DecodeString(req.RawTx)
		if err != nil {
			resp.Error = restful.INTERNAL_ERROR
			resp.Desc = fmt.Sprintf("Failed to decode hex transaction: %v", err)
			log.Errorf("BroadcastTx: decode rawtx from string to bytes failed, err: %s", err)
		} else if err = mtx.BtcDecode(bytes.NewBuffer(rawtx), wire.ProtocolVersion, wire.LatestEncoding); err != nil {
			resp.Error = restful.INTERNAL_ERROR
			resp.Desc = fmt.Sprintf("Failed to decode rawtx: %v", err)
			log.Errorf("BroadcastTx: decode rawtx failed, err: %s", err)
		} else if err = serv.wallet.Broadcast(mtx); err != nil {
			resp.Error = restful.INTERNAL_ERROR
			resp.Desc = fmt.Sprintf("Failed to broadcast transaction: %v", err)
			log.Errorf("BroadcastTx: broadcast failed, err: %s", err)
		} else {
			txid = mtx.TxHash().String()
			resp.Error = restful.SUCCESS
			resp.Result = nil
		}
	}

	m, err := utils.RefactorResp(resp, resp.Error)
	if err != nil {
		log.Errorf("BroadcastTx: failed, err: %s", err)
	} else {
		log.Infof("BroadcastTx: resp success, broadcast transaction %s", txid)
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

func (serv *Service) DelUtxo(params map[string]interface{}) map[string]interface{} {
	req := &common.DelUtxoReq{}
	resp := &common.Response{}

	err := utils.ParseParams(req, params)
	if err != nil {
		resp.Error = restful.INVALID_PARAMS
		resp.Desc = err.Error()
		log.Errorf("DelUtxo: decode params failed, err: %s", err)
	} else {
		hash, _ := chainhash.NewHashFromStr(req.Op.Hash)
		err := serv.wallet.GetTxStore().Utxos().Delete(wallet.Utxo{
			Op: wire.OutPoint{
				Hash:  *hash,
				Index: req.Op.Index,
			},
		})

		if err != nil {
			resp.Error = restful.INTERNAL_ERROR
			resp.Desc = err.Error()
			log.Errorf("DelUtxo: failed to delete %s:%d: %v", req.Op.Hash, req.Op.Index, err)
		} else {
			resp.Error = restful.SUCCESS
		}
	}

	m, err := utils.RefactorResp(resp, resp.Error)
	if err != nil {
		log.Errorf("DelUtxo: failed, err: %s", err)
	} else {
		log.Infof("DelUtxo: resp success, delete utxo %s:%d", req.Op.Hash, req.Op.Index)
	}
	return m
}
