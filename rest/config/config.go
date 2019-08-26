package config

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
)

const (
	DEFAULT_LOG_LEVEL        = 2
	DEFAULT_CONFIG_FILE_NAME = "./rest_conf.json"
)

//Config object used by ontology-instance
type Config struct {
	Port uint64 `json:"port"`
	Path string `json:"path"`
}

func NewConfig(fileName string) (*Config, error) {
	data, err := ioutil.ReadFile(fileName)
	if err != nil {
		return nil, err
	}
	cfg := &Config{}
	err = json.Unmarshal(data, cfg)
	if err != nil {
		return nil, fmt.Errorf("json.Unmarshal Config:%s error:%s", data, err)
	}
	return cfg, nil
}
