package spvwallet

import (
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/mitchellh/go-homedir"
	"github.com/urfave/cli"
	"golang.org/x/net/proxy"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

const (
	DEFAULT_LOG_LEVEL        = 2
	DEFAULT_CONFIG_FILE_NAME = "./rest_conf.json"
)

var (
	LogLevelFlag = cli.UintFlag{
		Name:  "loglevel",
		Usage: "Set the log level to `<level>` (0~6). 0:Trace 1:Debug 2:Info 3:Warn 4:Error 5:Fatal 6:MaxLevel",
		Value: DEFAULT_LOG_LEVEL,
	}

	ConfigBitcoinNet = cli.StringFlag{
		Name:  "nettype",
		Usage: "bitcoin net type: main, test, sim, regtest.",
		Value: "mian",
	}

	ConfigDBPath = cli.StringFlag{
		Name:  "dbpath",
		Usage: "config db path.",
		Value: "./spv_db",
	}

	TrustedPeer = cli.StringFlag{
		Name:  "trustedpeer",
		Usage: "the node you trust. default null",
		Value: "",
	}

	AlliaConfigFile = cli.StringFlag{
		Name:  "alliaconfig",
		Usage: "the config file of alliance service.",
		Value: "./allia_conf.json",
	}

	GoMaxProcs = cli.IntFlag{
		Name:  "gomaxprocs",
		Usage: "max number of cpu core that runtime can use.",
		Value: 4,
	}

	RunRest = cli.IntFlag{
		Name:  "rest",
		Usage: "1: start the restful service. 0: not start",
		Value: 1,
	}

	RunVote = cli.IntFlag{
		Name:  "vote",
		Usage: "1: start the vote service. 0: not start",
		Value: 1,
	}

	IsRestart = cli.IntFlag{
		Name:  "isrestart",
		Usage: "When the header is not updated, restrat the service or not. 1 means YES, 0 means NO, default 1",
		Value: 1,
	}

	RestartDuration = cli.IntFlag{
		Name:  "restart",
		Usage: "When the header is not updated for 'restart' min, restrat the service. default 15 min",
		Value: 15,
	}

	RestConfigPathFlag = cli.StringFlag{
		Name:  "restconfig",
		Usage: "rest server config file `<path>`",
		Value: DEFAULT_CONFIG_FILE_NAME,
	}
)

//GetFlagName deal with short flag, and return the flag name whether flag name have short name
func GetFlagName(flag cli.Flag) string {
	name := flag.GetName()
	if name == "" {
		return ""
	}
	return strings.TrimSpace(strings.Split(name, ",")[0])
}

type Config struct {
	// Network parameters. Set mainnet, testnet, or regtest using this.
	Params *chaincfg.Params

	// Bip39 mnemonic string. If empty a new mnemonic will be created.
	//Mnemonic string

	// The date the wallet was created.
	// If before the earliest checkpoint the chain will be synced using the earliest checkpoint.
	//CreationDate time.Time

	// The user-agent that shall be visible to peers
	UserAgent string

	// Location of the data directory
	RepoPath string

	// An implementation of the Datastore interface
	//DB wallet.Datastore

	// If you wish to connect to a single trusted peer set this. Otherwise leave nil.
	TrustedPeer net.Addr

	// A Tor proxy can be set here causing the wallet will use Tor
	Proxy proxy.Dialer

	// The default fee-per-byte for each level
	//LowFee    uint64
	//MediumFee uint64
	//HighFee   uint64

	// The highest allowable fee-per-byte
	//MaxFee uint64

	// External API to query to look up fees. If this field is nil then the default fees will be used.
	// If the API is unreachable then the default fees will likewise be used. If the API returns a fee
	// greater than MaxFee then the MaxFee will be used in place. The API response must be formatted as
	// { "fastestFee": 40, "halfHourFee": 20, "hourFee": 10 }
	//FeeAPI url.URL

	// A logger. You can write the logs to file or stdout or however else you want.
	//Logger logging.Backend

	// Disable the exchange rate provider
	//DisableExchangeRates bool
	IsVote bool
}

func NewDefaultConfig() *Config {
	repoPath, _ := getRepoPath()
	_, ferr := os.Stat(repoPath)
	if os.IsNotExist(ferr) {
		os.Mkdir(repoPath, os.ModePerm)
	}
	//feeApi, _ := url.Parse("https://bitcoinfees.earn.com/api/v1/fees/recommended")
	return &Config{
		IsVote:    false,
		Params:    &chaincfg.MainNetParams,
		UserAgent: "spvwallet",
		RepoPath:  repoPath,
		//LowFee:    20,
		//MediumFee: 30,
		//HighFee:   40,
		//MaxFee:    2000,
		//FeeAPI:    *feeApi,
		//Logger:    logging.NewLogBackend(os.Stdout, "", 0),
	}
}

func getRepoPath() (string, error) {
	// Set default base path and directory name
	path := "~"
	directoryName := "spvwallet"

	// Override OS-specific names
	switch runtime.GOOS {
	case "linux":
		directoryName = ".spvwallet"
	case "darwin":
		path = "~/"
	}

	// Join the path and directory name, then expand the home path
	fullPath, err := homedir.Expand(filepath.Join(path, directoryName))
	if err != nil {
		return "", err
	}

	// Return the shortest lexical representation of the path
	return filepath.Clean(fullPath), nil
}

//Config object used by ontology-instance
//type RestConfig struct {
//	Port uint64 `json:"port"`
//	Path string `json:"path"`
//}
//
//func NewRestConfig(fileName string) (*RestConfig, error) {
//	data, err := ioutil.ReadFile(fileName)
//	if err != nil {
//		return nil, err
//	}
//	cfg := &RestConfig{}
//	err = json.Unmarshal(data, cfg)
//	if err != nil {
//		return nil, fmt.Errorf("json.Unmarshal Config:%s error:%s", data, err)
//	}
//	return cfg, nil
//}
