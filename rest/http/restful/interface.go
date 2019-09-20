package restful

type Web interface {
	QueryHeaderByHeight(map[string]interface{}) map[string]interface{}
	GetCurrentHeight(map[string]interface{}) map[string]interface{}
	Rollback(params map[string]interface{}) map[string]interface{}
}
