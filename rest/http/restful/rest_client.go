/*
 * Copyright (C) 2018 The ontology Authors
 * This file is part of The ontology library.
 *
 * The ontology is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Lesser General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * The ontology is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Lesser General Public License for more details.
 *
 * You should have received a copy of the GNU Lesser General Public License
 * along with The ontology.  If not, see <http://www.gnu.org/licenses/>.
 */

package restful

import (
	"bytes"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/btcsuite/btcd/btcjson"
	"github.com/btcsuite/btcd/wire"
	"io/ioutil"
	"net/http"
	"time"
)

type QueryHeaderByHeightParam struct {
	Height uint32 `json:"height"`
}

type QueryUtxosReq struct {
	Addr      string `json:"addr"`
	Amount    int64  `json:"amount"`
	Fee       int64  `json:"fee"`
	IsPreExec bool   `json:"is_pre_exec"`
}

type ChangeAddressReq struct {
	Aciton string `json:"aciton"`
	Addr   string `json:"addr"`
}

type BroadcastTxReq struct {
	RawTx string `json:"raw_tx"`
}

type UnlockUtxoReq struct {
	Hash  string `json:"hash"`
	Index uint32 `json:"index"`
}

type GetFeePerByteReq struct {
	Level int `json:"level"`
}

type RollbackReq struct {
	Time string `json:"time"`
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

// response
type Response struct {
	Action string      `json:"action"`
	Desc   string      `json:"desc"`
	Error  uint32      `json:"error"`
	Result interface{} `json:"result"`
}

type ResponseAllUtxos struct {
	Action string          `json:"action"`
	Desc   string          `json:"desc"`
	Error  uint32          `json:"error"`
	Result GetAllUtxosResp `json:"result"`
}

type RestClient struct {
	Addr       string
	restClient *http.Client
}

func NewRestClient(addr string) *RestClient {
	return &RestClient{
		restClient: &http.Client{
			Transport: &http.Transport{
				MaxIdleConnsPerHost:   5,
				DisableKeepAlives:     false,
				IdleConnTimeout:       time.Second * 300,
				ResponseHeaderTimeout: time.Second * 300,
				TLSClientConfig:       &tls.Config{InsecureSkipVerify: true},
			},
			Timeout: time.Second * 300,
		},
		Addr: addr,
	}
}

func (self *RestClient) SetAddr(addr string) *RestClient {
	self.Addr = addr
	return self
}

func (self *RestClient) SetRestClient(restClient *http.Client) *RestClient {
	self.restClient = restClient
	return self
}

func (self *RestClient) SendRestRequest(addr string, data []byte) ([]byte, error) {
	resp, err := self.restClient.Post(addr, "application/json;charset=UTF-8",
		bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("rest post request:%s error:%s", data, err)
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read rest response body error:%s", err)
	}
	return body, nil
}

func (self *RestClient) SendGetRequst(addr string) ([]byte, error) {
	resp, err := self.restClient.Get(addr)
	if err != nil {
		return nil, fmt.Errorf("rest get request: error: %v", err)
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read get response body error:%s", err)
	}
	return body, nil
}

func (self *RestClient) GetHeaderFromSpv(height uint32) (*wire.BlockHeader, error) {
	query, err := json.Marshal(QueryHeaderByHeightParam{
		Height: height,
	})
	if err != nil {
		return nil, fmt.Errorf("Failed to parse query parameter: %v", err)
	}

	// how to config it???
	data, err := self.SendRestRequest("http://"+self.Addr+"/api/v1/queryheaderbyheight", query)
	if err != nil {
		return nil, fmt.Errorf("Failed to send request: %v", err)
	}
	var resp Response
	err = json.Unmarshal(data, &resp)
	if err != nil {
		return nil, fmt.Errorf("Failed to unmarshal resp to json: %v", err)
	}

	if resp.Error != 0 || resp.Desc != "SUCCESS" {
		return nil, fmt.Errorf("Response shows failure: %s", resp.Desc)
	}

	hbs, err := hex.DecodeString(resp.Result.(map[string]interface{})["header"].(string))
	if err != nil {
		return nil, fmt.Errorf("Failed to decode hex string from response: %v", err)
	}

	header := wire.BlockHeader{}
	buf := bytes.NewReader(hbs)
	err = header.BtcDecode(buf, wire.ProtocolVersion, wire.LatestEncoding)
	if err != nil {
		return nil, fmt.Errorf("Failed to decode header: %v", err)
	}
	return &header, nil
}

func (self *RestClient) GetUtxosFromSpv(addr string, amount int64, fee int64, isPreExec bool) ([]btcjson.TransactionInput, int64, error) {
	query, err := json.Marshal(QueryUtxosReq{
		Addr:      addr,
		Amount:    amount,
		Fee:       fee,
		IsPreExec: isPreExec,
	})
	if err != nil {
		return nil, 0, fmt.Errorf("Failed to parse parameter: %v", err)
	}
	data, err := self.SendRestRequest("http://"+self.Addr+"/api/v1/queryutxos", query)
	if err != nil {
		return nil, 0, fmt.Errorf("Failed to send request: %v", err)
	}

	var resp Response
	err = json.Unmarshal(data, &resp)
	if err != nil {
		return nil, 0, fmt.Errorf("Failed to unmarshal resp to json: %v", err)
	}
	if resp.Error != 0 || resp.Desc != "SUCCESS" {
		return nil, 0, fmt.Errorf("Response shows failure: %s", resp.Desc)
	}
	var ins []btcjson.TransactionInput
	for _, v := range resp.Result.(map[string]interface{})["inputs"].([]interface{}) {
		m := v.(map[string]interface{})
		ins = append(ins, btcjson.TransactionInput{
			Txid: m["txid"].(string),
			Vout: uint32(m["vout"].(float64)),
		})
	}

	return ins, int64(resp.Result.(map[string]interface{})["sum"].(float64)), nil
}

func (self *RestClient) GetCurrentHeightFromSpv() (uint32, error) {
	data, err := self.SendGetRequst("http://" + self.Addr + "/api/v1/getcurrentheight")
	if err != nil {
		return 0, fmt.Errorf("Failed to send request: %v", err)
	}

	var resp Response
	err = json.Unmarshal(data, &resp)
	if err != nil {
		return 0, fmt.Errorf("Failed to unmarshal resp to json: %v", err)
	}
	if resp.Error != 0 || resp.Desc != "SUCCESS" {
		return 0, fmt.Errorf("Response shows failure: %s", resp.Desc)
	}

	return uint32(resp.Result.(map[string]interface{})["height"].(float64)), nil
}

func (self *RestClient) ChangeSpvWatchedAddr(addr string, action string) error {
	req, err := json.Marshal(ChangeAddressReq{
		Addr:   addr,
		Aciton: action,
	})
	if err != nil {
		return fmt.Errorf("Failed to parse parameter: %v", err)
	}
	data, err := self.SendRestRequest("http://"+self.Addr+"/api/v1/changeaddress", req)
	if err != nil {
		return fmt.Errorf("Failed to send request: %v", err)
	}

	var resp Response
	err = json.Unmarshal(data, &resp)
	if err != nil {
		return fmt.Errorf("Failed to unmarshal resp to json: %v", err)
	}
	if resp.Error != 0 || resp.Desc != "SUCCESS" {
		return fmt.Errorf("Response shows failure: %s", resp.Desc)
	}

	return nil
}

func (self *RestClient) GetWatchedAddrsFromSpv() ([]string, error) {
	data, err := self.SendGetRequst("http://" + self.Addr + "/api/v1/getalladdress")
	if err != nil {
		return nil, fmt.Errorf("Failed to send request: %v", err)
	}

	var resp Response
	err = json.Unmarshal(data, &resp)
	if err != nil {
		return nil, fmt.Errorf("Failed to unmarshal resp to json: %v", err)
	}
	if resp.Error != 0 || resp.Desc != "SUCCESS" {
		return nil, fmt.Errorf("Response shows failure: %s", resp.Desc)
	}
	var addrs []string
	for _, v := range resp.Result.(map[string]interface{})["addresses"].([]interface{}) {
		addrs = append(addrs, v.(string))
	}
	return addrs, nil
}

func (self *RestClient) UnlockUtxoInSpv(hash string, index uint32) error {
	req, err := json.Marshal(UnlockUtxoReq{
		Hash:  hash,
		Index: index,
	})
	if err != nil {
		return fmt.Errorf("Failed to parse parameter: %v", err)
	}
	data, err := self.SendRestRequest("http://"+self.Addr+"/api/v1/unlockutxo", req)
	if err != nil {
		return fmt.Errorf("Failed to send request: %v", err)
	}

	var resp Response
	err = json.Unmarshal(data, &resp)
	if err != nil {
		return fmt.Errorf("Failed to unmarshal resp to json: %v", err)
	}
	if resp.Error != 0 || resp.Desc != "SUCCESS" {
		return fmt.Errorf("Response shows failure: %s", resp.Desc)
	}

	return nil
}

func (self *RestClient) GetFeeRateFromSpv(level int) (int64, error) {
	req, err := json.Marshal(GetFeePerByteReq{
		Level: level,
	})
	if err != nil {
		return -1, fmt.Errorf("Failed to parse parameter: %v", err)
	}

	data, err := self.SendRestRequest("http://"+self.Addr+"/api/v1/getfeeperbyte", req)
	if err != nil {
		return -1, fmt.Errorf("Failed to send request: %v", err)
	}

	var resp Response
	err = json.Unmarshal(data, &resp)
	if err != nil {
		return -1, fmt.Errorf("Failed to unmarshal resp to json: %v", err)
	}
	if resp.Error != 0 || resp.Desc != "SUCCESS" {
		return -1, fmt.Errorf("Response shows failure: %s", resp.Desc)
	}

	return int64(resp.Result.(map[string]interface{})["feepb"].(float64)), nil
}

func (self *RestClient) GetAllUtxosFromSpv() ([]UtxoInfo, error) {
	data, err := self.SendGetRequst("http://" + self.Addr + "/api/v1/getallutxos")
	if err != nil {
		return nil, fmt.Errorf("Failed to send request: %v", err)
	}

	var resp ResponseAllUtxos
	err = json.Unmarshal(data, &resp)
	if err != nil {
		return nil, fmt.Errorf("Failed to unmarshal resp to json: %v", err)
	}
	if resp.Error != 0 || resp.Desc != "SUCCESS" {
		return nil, fmt.Errorf("Response shows failure: %s", resp.Desc)
	}

	return resp.Result.Infos, nil
}

func (self *RestClient) BroadcastTxBySpv(mtx *wire.MsgTx) error {
	var buf bytes.Buffer
	err := mtx.BtcEncode(&buf, wire.ProtocolVersion, wire.LatestEncoding)
	if err != nil {
		return err
	}
	req, err := json.Marshal(BroadcastTxReq{
		RawTx: hex.EncodeToString(buf.Bytes()),
	})
	if err != nil {
		return fmt.Errorf("Failed to parse parameter: %v", err)
	}

	data, err := self.SendRestRequest("http://"+self.Addr+"/api/v1/broadcasttx", req)
	if err != nil {
		return fmt.Errorf("Failed to send request: %v", err)
	}

	var resp Response
	err = json.Unmarshal(data, &resp)
	if err != nil {
		return fmt.Errorf("Failed to unmarshal resp to json: %v", err)
	}
	if resp.Error != 0 || resp.Desc != "SUCCESS" {
		return fmt.Errorf("Response shows failure: %s", resp.Desc)
	}

	return nil
}

func (self *RestClient) RollbackSpv(time string) error {
	req, err := json.Marshal(RollbackReq{
		Time: time,
	})
	if err != nil {
		return fmt.Errorf("Failed to parse parameter: %v", err)
	}
	data, err := self.SendRestRequest("http://"+self.Addr+"/api/v1/rollback", req)
	if err != nil {
		return fmt.Errorf("Failed to send request: %v", err)
	}

	var resp Response
	err = json.Unmarshal(data, &resp)
	if err != nil {
		return fmt.Errorf("Failed to unmarshal resp to json: %v", err)
	}
	if resp.Error != 0 || resp.Desc != "SUCCESS" {
		return fmt.Errorf("Response shows failure: %s", resp.Desc)
	}

	return nil
}
