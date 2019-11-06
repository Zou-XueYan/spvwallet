package config

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
)

type Config struct {
	AllianceJsonRpcAddress string
	GasPrice               uint64
	GasLimit               uint64
	WalletFile             string
	WalletPwd              string
	AlliaObFirstN          int64 // AlliaOb:
	AlliaObLoopWaitTime    int64
	WatchingKey            string
	Redeem                 string
	WaitingDBPath          string
	BlksToWait             uint64
	BtcPrivk               string
	WatchingMakeTxKey      string
	ConfigBitcoinNet       string
	ConfigDBPath           string
	TrustedPeer            string
	RunRest                int
	RestConfigPath         string
	RunVote                int
	RestartDuration        int
	IsRestart              int
	RestPath               string
	RestPort               uint64
}

func NewConfig(file string) (*Config, error) {
	conf := &Config{}
	err := conf.Init(file)
	if err != nil {
		return conf, fmt.Errorf("failed to new config: %v", err)
	}
	return conf, nil
}

func (this *Config) Init(fileName string) error {
	err := this.loadConfig(fileName)
	if err != nil {
		return fmt.Errorf("loadConfig error:%s", err)
	}
	return nil
}

func (this *Config) loadConfig(fileName string) error {
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

func (this *Config) readFile(fileName string) ([]byte, error) {
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
