package common

const (
	QUERYHEADERBYHEIGHT = "/api/v1/queryheaderbyheight"
	GETCURRENTHEIGHT    = "/api/v1/getcurrentheight"
	ROLLBACK            = "/api/v1/rollback"
	BROADCASTTX         = "/api/v1/broadcasttx"
)

const (
	ACTION_QUERYHEADERBYHEIGHT = "queryheaderbyheight"
	ACTION_GETCURRENTHEIGHT    = "getcurrentheight"
	ACTION_ROLLBACK            = "rollback"
	ACTION_BROADCASTTX         = "broadcasttx"
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

type GetCurrentHeightResp struct {
	Height uint32 `json:"height"`
}

type RollbackReq struct {
	Time string `json:"time"`
}

type BroadcastReq struct {
	Tx string `json:"tx"`
}
