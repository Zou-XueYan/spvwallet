package main

import (
	"encoding/hex"
	"fmt"
	"github.com/Zou-XueYan/spvwallet"
	"github.com/Zou-XueYan/spvwallet/alliance"
	"github.com/Zou-XueYan/spvwallet/db"
	"github.com/Zou-XueYan/spvwallet/rest/config"
	"github.com/Zou-XueYan/spvwallet/rest/http/restful"
	"github.com/Zou-XueYan/spvwallet/rest/log"
	"github.com/Zou-XueYan/spvwallet/rest/service"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcutil"
	"github.com/ontio/multi-chain/smartcontract/service/native/cross_chain_manager/btc"
	sdk "github.com/ontio/ontology-go-sdk"
	"github.com/op/go-logging"
	"github.com/urfave/cli"
	"net"
	"os"
	"os/signal"
	"path"
	"runtime"
	"syscall"
	"time"
)

func setupApp() *cli.App {
	app := cli.NewApp()
	app.Usage = "start spv client with restful service"
	app.Action = startSpvClient
	app.Copyright = ""
	app.Flags = []cli.Flag{
		config.RestLogLevelFlag,
		config.RestConfigPathFlag,
		config.ConfigBitcoinNet,
		config.ConfigDBPath,
		config.WalletCreatedTime,
		config.TrustedPeer,
		config.WatchedAddress,
		config.AlliaConfigFile,
		config.GoMaxProcs,
		config.RunRest,
	}
	app.Before = func(context *cli.Context) error {
		cores := context.GlobalInt(config.GoMaxProcs.Name)
		runtime.GOMAXPROCS(cores) //(runtime.NumCPU())
		return nil
	}
	return app
}

func startSpvClient(ctx *cli.Context) {
	backend := logging.NewLogBackend(os.Stdout, "", 0)
	formatter := logging.MustStringFormatter(`%{color}%{time:2006/01/02 15:04:05} [%{shortfunc}] [%{level}] %{message}`)
	stdoutFormatter := logging.NewBackendFormatter(backend, formatter)

	conf := spvwallet.NewDefaultConfig()
	conf.Logger = logging.MultiLogger(stdoutFormatter)
	netType := ctx.GlobalString(config.GetFlagName(config.ConfigBitcoinNet))

	dbPath := ctx.GlobalString(config.GetFlagName(config.ConfigDBPath))
	if dbPath != "" {
		conf.RepoPath = dbPath
	}

	switch netType {
	case "regtest":
		conf.Params = &chaincfg.RegressionNetParams
		conf.RepoPath = path.Join(conf.RepoPath, "regtest")
	case "test":
		conf.Params = &chaincfg.TestNet3Params
	case "sim":
		conf.Params = &chaincfg.SimNetParams
	default:
		conf.Params = &chaincfg.MainNetParams
	}

	tp := ctx.GlobalString(config.GetFlagName(config.TrustedPeer))
	if tp != "" {
		conf.TrustedPeer, _ = net.ResolveTCPAddr("tcp", tp+":"+conf.Params.DefaultPort)
	}

	sqliteDatastore, err := db.Create(conf.RepoPath)
	if err != nil {
		log.Fatalf("Failed to create db: %v", err)
		return
	}
	conf.DB = sqliteDatastore

	createdTime, err := time.Parse("2006-01-02 15:04:05", ctx.GlobalString(config.GetFlagName(config.WalletCreatedTime)))
	if err != nil {
		log.Fatalf("Failed to parse WalletCreatedTime, please check your input %s: %v",
			ctx.GlobalString(config.GetFlagName(config.WalletCreatedTime)), err)
		return
	}
	conf.CreationDate = createdTime
	log.Infof("Set wallet created time %s", createdTime.String())

	conf.DisableExchangeRates = true
	watchedAddr := ctx.GlobalString(config.GetFlagName(config.WatchedAddress))

	wallet, _ := spvwallet.NewSPVWallet(conf)
	if watchedAddr != "" {
		wa, err := btcutil.DecodeAddress(watchedAddr, conf.Params)
		if err != nil {
			log.Fatalf("Failed to decode your watched address %s: %v", watchedAddr, err)
			return
		}
		wallet.AddWatchedAddress(wa)
		log.Infof("Add %s to watched address", watchedAddr)
	}
	wallet.Start()
	defer wallet.Close()

	logLevel := ctx.GlobalInt(config.GetFlagName(config.RestLogLevelFlag))
	log.InitLog(logLevel, log.Stdout)
	if ctx.GlobalInt(config.RunRest.Name) == 1 {
		err = startServer(ctx, wallet)
		if err != nil {
			log.Fatalf("Failed to start rest service: %v", err)
			return
		}
	}

	err = startAllianceService(ctx, wallet)
	if err != nil {
		log.Fatalf("Failed to start alliance service: %v", err)
	}

	waitToExit()
}

func startAllianceService(ctx *cli.Context, wallet *spvwallet.SPVWallet) error {
	conf, err := alliance.NewAlliaConfig(ctx.GlobalString(config.GetFlagName(config.AlliaConfigFile)))
	if err != nil {
		return err
	}
	voting := make(chan *btc.BtcProof, 10)
	allia := sdk.NewOntologySdk()
	allia.NewRpcClient().SetAddress(conf.AllianceJsonRpcAddress)
	acct, err := alliance.GetAccountByPassword(allia, conf.WalletFile, conf.WalletPwd)
	if err != nil {
		return fmt.Errorf("GetAccountByPassword failed: %v", err)
	}

	ob := alliance.NewObserver(allia, &alliance.ObConfig{
		FirstN:       conf.AlliaObFirstN,
		LoopWaitTime: conf.AlliaObLoopWaitTime,
		WatchingKey:  conf.WatchingKey,
	}, voting)
	go ob.Listen()

	redeem, err := hex.DecodeString(conf.Redeem)
	if err != nil {
		return fmt.Errorf("failed to decode redeem %s: %v", conf.Redeem, err)
	}
	v, err := alliance.NewVoter(allia, voting, wallet, redeem, acct, conf.GasPrice, conf.GasLimit, conf.WaitingDBPath,
		conf.BlksToWait)

	go v.Vote()
	go v.WaitingRetry()

	return nil
}

func startServer(ctx *cli.Context, wallet *spvwallet.SPVWallet) error {
	configPath := ctx.GlobalString(config.GetFlagName(config.RestConfigPathFlag))
	servConfig, err := config.NewConfig(configPath)
	if err != nil {
		return err
	}

	serv := service.NewService(wallet, servConfig)
	restServer := restful.InitRestServer(serv, servConfig.Port)
	go restServer.Start()
	//go checkLogFile(logLevel)

	return nil
}

func waitToExit() {
	exit := make(chan bool, 0)
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)
	go func() {
		for sig := range sc {
			log.Infof("server received exit signal:%v.", sig.String())
			close(exit)
			break
		}
	}()
	<-exit
}

func checkLogFile(logLevel int) {
	ticker := time.NewTicker(5 * time.Second)
	for {
		select {
		case <-ticker.C:
			isNeedNewFile := log.CheckIfNeedNewFile()
			if isNeedNewFile {
				log.ClosePrintLog()
				log.InitLog(logLevel, log.PATH, log.Stdout)
			}
		}
	}
}

func main() {
	if err := setupApp().Run(os.Args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
