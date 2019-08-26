package restful

type Web interface {
	QueryHeaderByHeight(map[string]interface{}) map[string]interface{}
	QueryUtxos(map[string]interface{}) map[string]interface{}
	GetCurrentHeight(map[string]interface{}) map[string]interface{}
	ChangeAddress(map[string]interface{}) map[string]interface{}
	GetAllAddress(map[string]interface{}) map[string]interface{}
	UnlockUtxo(map[string]interface{}) map[string]interface{}
	GetFeePerByte(map[string]interface{}) map[string]interface{}
	GetAllUtxos(map[string]interface{}) map[string]interface{}
	BroadcastTx(map[string]interface{}) map[string]interface{}
	Rollback(params map[string]interface{}) map[string]interface{}
	DelUtxo(params map[string]interface{}) map[string]interface{}
}
