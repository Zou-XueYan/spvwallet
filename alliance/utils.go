package alliance

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/btcsuite/btcd/wire"
	"github.com/ontio/multi-chain/common"
	sdk "github.com/ontio/ontology-go-sdk"
	"io/ioutil"
	"os"
)

const (
	OP_RETURN_DATA_LEN    = 37
	OP_RETURN_SCRIPT_FLAG = byte(0x66)
	BTC_CHAINID           = 0
	MIN_FEE               = 100 //TODO: set one?
)

type ToVoteItem struct {
	MsgTx        *wire.MsgTx
	Height       uint32
	Proof        []byte
	BlocksToWait uint64
}

// func about OP_RETURN
func ifCanResolve(paramOutput *wire.TxOut, value int64) error {
	script := paramOutput.PkScript
	if int(script[1]) != OP_RETURN_DATA_LEN {
		return errors.New("length of script is wrong")
	}
	if script[2] != OP_RETURN_SCRIPT_FLAG {
		return errors.New("wrong flag")
	}
	_ = binary.BigEndian.Uint64(script[3:11])
	fee := int64(binary.BigEndian.Uint64(script[11:19]))
	//if fee < MIN_FEE {
	//	return fmt.Errorf("transaction fee %d is less than minimum transaction fee %d", fee, MIN_FEE)
	//}
	_, err := common.AddressParseFromBytes(script[19:])
	if err != nil {
		return fmt.Errorf("failed to parse address from bytes: %v", err)
	}
	if value < fee && fee >= 0 {
		return errors.New("the transfer amount cannot be less than the transaction fee")
	}
	return nil
}

type AlliaConfig struct {
	AllianceJsonRpcAddress string
	GasPrice               uint64
	GasLimit               uint64
	WalletFile             string
	WalletPwd              string
	AlliaObFirstN          int // AlliaOb:
	AlliaObLoopWaitTime    int64
	WatchingKey            string
	Redeem                 string
	WaitingDBPath          string
	BlksToWait             uint64
}

func NewAlliaConfig(file string) (*AlliaConfig, error) {
	conf := &AlliaConfig{}
	err := conf.Init(file)
	if err != nil {
		return conf, fmt.Errorf("failed to new config: %v", err)
	}
	return conf, nil
}

func (this *AlliaConfig) Init(fileName string) error {
	err := this.loadConfig(fileName)
	if err != nil {
		return fmt.Errorf("loadConfig error:%s", err)
	}
	return nil
}

func (this *AlliaConfig) loadConfig(fileName string) error {
	data, err := this.readFile(fileName)
	if err != nil {
		return err
	}
	err = json.Unmarshal(data, this)
	if err != nil {
		return fmt.Errorf("json.Unmarshal TestConfig:%s error:%s", data, err)
	}
	return nil
}

func (this *AlliaConfig) readFile(fileName string) ([]byte, error) {
	file, err := os.OpenFile(fileName, os.O_RDONLY, 0666)
	if err != nil {
		return nil, fmt.Errorf("OpenFile %s error %s", fileName, err)
	}
	defer func() {
		err := file.Close()
		if err != nil {
			fmt.Println(fmt.Errorf("file %s close error %s", fileName, err))
		}
	}()
	data, err := ioutil.ReadAll(file)
	if err != nil {
		return nil, fmt.Errorf("ioutil.ReadAll %s error %s", fileName, err)
	}
	return data, nil
}

func GetAccountByPassword(sdk *sdk.OntologySdk, path, pwd string) (*sdk.Account, error) {
	wallet, err := sdk.OpenWallet(path)
	if err != nil {
		return nil, fmt.Errorf("open wallet error: %v", err)
	}
	user, err := wallet.GetDefaultAccount([]byte(pwd))
	if err != nil {
		return nil, fmt.Errorf("getDefaultAccount error: %v", err)
	}
	return user, nil
}

type LessConfirmationError struct {
	Err error
}

func (err LessConfirmationError) Error() string {
	return err.String()
}

func (err *LessConfirmationError) String() string {
	return err.Err.Error()
}
