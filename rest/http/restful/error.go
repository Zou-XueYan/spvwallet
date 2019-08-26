// Package error privides error code for rest
package restful

const (
	SUCCESS uint32 = 0
	FAILED  uint32 = 1

	INVALID_METHOD     uint32 = 42001
	INVALID_PARAMS     uint32 = 42002
	ILLEGAL_DATAFORMAT uint32 = 42003
	INTERNAL_ERROR     uint32 = 42004
)

var ErrMap = map[uint32]string{
	SUCCESS:            "SUCCESS",
	FAILED:             "FAILED",
	INVALID_METHOD:     "INVALID METHOD",
	INVALID_PARAMS:     "INVALID PARAMS",
	ILLEGAL_DATAFORMAT: "ILLEGAL DATAFORMAT",
	INTERNAL_ERROR:     "INTERNAL_ERROR",
}
