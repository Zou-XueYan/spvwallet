package restful

func PackResponse(errCode uint32) map[string]interface{} {
	resp := map[string]interface{}{
		"action": "",
		"result": "",
		"error":  errCode,
		"desc":   "",
	}
	return resp
}
