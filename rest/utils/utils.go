package utils

import (
	"encoding/json"
	"fmt"
	"github.com/Zou-XueYan/spvwallet"
	"github.com/Zou-XueYan/spvwallet/rest/http/common"
	"github.com/btcsuite/btcd/wire"
)

func ParseParams(req interface{}, params map[string]interface{}) error {
	jsonData, err := json.Marshal(params)
	if err != nil {
		return fmt.Errorf("ParseParams: marshal params failed, err: %s", err)
	}
	err = json.Unmarshal(jsonData, req)
	if err != nil {
		return fmt.Errorf("ParseParams: unmarshal req failed, err: %s", err)
	}
	return nil
}

func RefactorResp(resp *common.Response, errCode uint32) (map[string]interface{}, error) {
	m := make(map[string]interface{})
	jsonResp, err := json.Marshal(resp)
	if err != nil {
		return m, fmt.Errorf("RefactorResp: marhsal resp failed, err: %s", err)
	}
	err = json.Unmarshal(jsonResp, &m)
	if err != nil {
		return m, fmt.Errorf("RefactorResp: unmarhsal resp failed, err: %s", err)
	}
	m["error"] = errCode
	return m, nil
}

// only for cross chain
func EstimateSerializedTxSize(inputCount int, txOuts []*wire.TxOut) int {
	multi5of7InputSize := 32 + 4 + 1 + 4 + spvwallet.RedeemP2SH5of7MultisigSigScriptSize

	outsSize := 0
	for _, txOut := range txOuts {
		outsSize += txOut.SerializeSize()
	}

	return 10 + wire.VarIntSerializeSize(uint64(inputCount)) + wire.VarIntSerializeSize(uint64(len(txOuts)+1)) +
		inputCount*multi5of7InputSize + spvwallet.MaxP2SHScriptSize + outsSize
}
