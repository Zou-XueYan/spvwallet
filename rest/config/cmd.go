package config

import (
	"github.com/urfave/cli"
	"strings"
)

var (
	LogLevelFlag = cli.UintFlag{
		Name:  "loglevel",
		Usage: "Set the log level to `<level>` (0~6). 0:Trace 1:Debug 2:Info 3:Warn 4:Error 5:Fatal 6:MaxLevel",
		Value: DEFAULT_LOG_LEVEL,
	}

	RestConfigPathFlag = cli.StringFlag{
		Name:  "restconfig",
		Usage: "rest server config file `<path>`",
		Value: DEFAULT_CONFIG_FILE_NAME,
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

	WalletCreatedTime = cli.StringFlag{
		Name:  "createdtime",
		Usage: "time format is '2009-01-09 02:54:25'.",
		Value: "2009-01-04 02:15:05",
	}

	TrustedPeer = cli.StringFlag{
		Name:  "trustedpeer",
		Usage: "the node you trust. default null",
		Value: "",
	}

	WatchedAddress = cli.StringFlag{
		Name:  "watchedaddr",
		Usage: "the address that spv client need to watch for maintaining utxo set.",
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
		Usage: "When the header is not updated for 'restart' min, restrat the service. default 10 min",
		Value: 15,
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
