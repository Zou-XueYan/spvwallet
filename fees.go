package spvwallet

import (
	"encoding/json"
	"github.com/Zou-XueYan/spvwallet/interface"
	"golang.org/x/net/proxy"
	"math"
	"net"
	"net/http"
	"time"
)

type httpClient interface {
	Get(string) (*http.Response, error)
}

type feeCache struct {
	fees        *Fees
	lastUpdated time.Time
}

type Fees struct {
	FastestFee  uint64 `json:"fastestFee"`
	HalfHourFee uint64 `json:"halfHourFee"`
	HourFee     uint64 `json:"hourFee"`
}

type FeeProvider struct {
	maxFee      uint64
	priorityFee uint64
	normalFee   uint64
	economicFee uint64
	feeAPI      string

	httpClient httpClient

	cache *feeCache
}

func NewFeeProvider(maxFee, priorityFee, normalFee, economicFee uint64, feeAPI string, proxy proxy.Dialer) *FeeProvider {
	fp := FeeProvider{
		maxFee:      maxFee,
		priorityFee: priorityFee,
		normalFee:   normalFee,
		economicFee: economicFee,
		feeAPI:      feeAPI,
		cache:       new(feeCache),
	}
	dial := net.Dial
	if proxy != nil {
		dial = proxy.Dial
	}
	tbTransport := &http.Transport{Dial: dial}
	httpClient := &http.Client{Transport: tbTransport, Timeout: time.Second * 10}
	fp.httpClient = httpClient
	return &fp
}

func (fp *FeeProvider) GetFeePerByte(feeLevel wallet.FeeLevel) uint64 {
	if fp.feeAPI == "" {
		return fp.defaultFee(feeLevel)
	}
	fees := new(Fees)
	if time.Since(fp.cache.lastUpdated) > time.Minute {
		resp, err := fp.httpClient.Get(fp.feeAPI)
		if err != nil {
			return fp.defaultFee(feeLevel)
		}

		defer resp.Body.Close()

		err = json.NewDecoder(resp.Body).Decode(&fees)
		if err != nil || fees.FastestFee > math.MaxInt64 || fees.HourFee > math.MaxInt64 ||
			fees.HalfHourFee > math.MaxInt64 {
			return fp.defaultFee(feeLevel)
		}
		fp.cache.lastUpdated = time.Now()
		fp.cache.fees = fees
	} else {
		fees = fp.cache.fees
	}
	switch feeLevel {
	case wallet.PRIOIRTY:
		return fp.selectFee(fees.FastestFee, wallet.PRIOIRTY)
	case wallet.NORMAL:
		return fp.selectFee(fees.HalfHourFee, wallet.PRIOIRTY)
	case wallet.ECONOMIC:
		return fp.selectFee(fees.HourFee, wallet.PRIOIRTY)
	case wallet.FEE_BUMP:
		return fp.selectFee(fees.HourFee, wallet.PRIOIRTY)
	default:
		return fp.normalFee
	}
}

func (fp *FeeProvider) selectFee(fee uint64, feeLevel wallet.FeeLevel) uint64 {
	if fee > fp.maxFee {
		return fp.maxFee
	} else if fee == 0 {
		return fp.defaultFee(feeLevel)
	} else {
		return fee
	}
}

func (fp *FeeProvider) defaultFee(feeLevel wallet.FeeLevel) uint64 {
	switch feeLevel {
	case wallet.PRIOIRTY:
		return fp.priorityFee
	case wallet.NORMAL:
		return fp.normalFee
	case wallet.ECONOMIC:
		return fp.economicFee
	case wallet.FEE_BUMP:
		return fp.priorityFee
	default:
		return fp.normalFee
	}
}
