package spvclient

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
	DEFAULT_LOG_LEVEL   = 2
	DEFAULT_MAXPROC_NUM = 4
)

var (
	LogLevelFlag = cli.UintFlag{
		Name:  "loglevel",
		Usage: "Set the log level to `<level>` (0~6). 0:Trace 1:Debug 2:Info 3:Warn 4:Error 5:Fatal 6:MaxLevel",
		Value: DEFAULT_LOG_LEVEL,
	}

	ConfigFile = cli.StringFlag{
		Name:  "config",
		Usage: "the config file of alliance service.",
		Value: "./conf.json",
	}

	GoMaxProcs = cli.IntFlag{
		Name:  "gomaxprocs",
		Usage: "max number of cpu core that runtime can use.",
		Value: DEFAULT_MAXPROC_NUM,
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

	// The user-agent that shall be visible to peers
	UserAgent string

	// Location of the data directory
	RepoPath string

	// If you wish to connect to a single trusted peer set this. Otherwise leave nil.
	TrustedPeer net.Addr

	// A Tor proxy can be set here causing the wallet will use Tor
	Proxy proxy.Dialer

	IsVote bool
}

func NewDefaultConfig() *Config {
	repoPath, _ := getRepoPath()
	_, ferr := os.Stat(repoPath)
	if os.IsNotExist(ferr) {
		os.Mkdir(repoPath, os.ModePerm)
	}
	return &Config{
		IsVote:    false,
		UserAgent: "spvclient",
		RepoPath:  repoPath,
	}
}

func getRepoPath() (string, error) {
	// Set default base path and directory name
	path := "~"
	directoryName := "spvclient"

	// Override OS-specific names
	switch runtime.GOOS {
	case "linux":
		directoryName = ".spvclient"
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
