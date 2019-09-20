package main

import (
	"encoding/hex"
	"fmt"
	"github.com/Zou-XueYan/spvwallet"
	"github.com/Zou-XueYan/spvwallet/alliance"
	"github.com/Zou-XueYan/spvwallet/log"
	"github.com/Zou-XueYan/spvwallet/rest/http/restful"
	"github.com/Zou-XueYan/spvwallet/rest/service"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/google/gops/agent"
	sdk "github.com/ontio/multi-chain-go-sdk"
	"github.com/ontio/multi-chain/native/service/cross_chain_manager/btc"
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
	app.Usage = "start spv client"
	app.Action = startSpvClient
	app.Copyright = ""
	app.Flags = []cli.Flag{
		spvwallet.LogLevelFlag,
		spvwallet.ConfigBitcoinNet,
		spvwallet.ConfigDBPath,
		spvwallet.TrustedPeer,
		spvwallet.AlliaConfigFile,
		spvwallet.GoMaxProcs,
		spvwallet.RunRest,
		spvwallet.RestConfigPathFlag,
		spvwallet.RunVote,
		spvwallet.RestartDuration,
		spvwallet.IsRestart,
	}
	app.Before = func(context *cli.Context) error {
		cores := context.GlobalInt(spvwallet.GoMaxProcs.Name)
		runtime.GOMAXPROCS(cores)
		return nil
	}
	return app
}

func startSpvClient(ctx *cli.Context) {
	logLevel := ctx.GlobalInt(spvwallet.GetFlagName(spvwallet.LogLevelFlag))
	log.InitLog(logLevel, log.Stdout)

	conf := spvwallet.NewDefaultConfig()
	isVote := ctx.GlobalInt(spvwallet.RunVote.Name) == 1
	conf.IsVote = isVote

	netType := ctx.GlobalString(spvwallet.GetFlagName(spvwallet.ConfigBitcoinNet))
	dbPath := ctx.GlobalString(spvwallet.GetFlagName(spvwallet.ConfigDBPath))
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

	tp := ctx.GlobalString(spvwallet.GetFlagName(spvwallet.TrustedPeer))
	if tp != "" {
		conf.TrustedPeer, _ = net.ResolveTCPAddr("tcp", tp+":"+conf.Params.DefaultPort)
	}

	wallet, _ := spvwallet.NewSPVWallet(conf)
	wallet.Start()
	defer wallet.Close()

	voting := make(chan *btc.BtcProof, 10)
	quit := make(chan struct{})

	var restServer restful.ApiServer
	var err error
	if ctx.GlobalInt(spvwallet.RunRest.Name) == 1 {
		restServer, err = startServer(ctx, wallet)
		if err != nil {
			log.Fatalf("Failed to start rest service: %v", err)
			os.Exit(1)
		}
	}

	if isVote {
		err := startAllianceService(ctx, wallet, voting, quit) // TODO:restart need update the wallet
		if err != nil {
			log.Fatalf("Failed to start alliance service: %v", err)
		}
	}

	sh, err := wallet.Blockchain.BestBlock()
	if err != nil {
		log.Fatalf("Failed to get best block: %v", err)
		os.Exit(1)
	}
	lasth := sh.Height

	if ctx.GlobalInt(spvwallet.IsRestart.Name) == 1 {
		again := false
		td := time.Duration(ctx.GlobalInt(spvwallet.RestartDuration.Name)) * time.Minute
		timer := time.NewTimer(td)
		for {
			<-timer.C
			sh, err = wallet.Blockchain.BestBlock()
			if err != nil {
				log.Fatalf("Failed to get best block: %v", err)
				continue
			}
			if lasth >= sh.Height {
				isrb := false
				log.Debugf("Restart now!!!")
				if isVote {
					log.Debugf("stop voter")
					close(quit)
				}
				if restServer != nil {
					log.Debugf("stop rest service")
					restServer.Stop()
				}

				if again {
					log.Debugf("It happened TWICE!!!")
					err = wallet.Blockchain.Rollback(sh.Header.Timestamp.Add(-6 * time.Hour))
					if err != nil {
						log.Fatalf("Failed to rollback: %v", err)
						continue
					}
					isrb = true
					_ = os.RemoveAll(path.Join(conf.RepoPath, "peers.json"))
				}
				wallet.Close()

				wallet, _ = spvwallet.NewSPVWallet(conf)
				wallet.Start()

				if ctx.GlobalInt(spvwallet.RunRest.Name) == 1 {
					restServer, err = startServer(ctx, wallet)
					if err != nil {
						log.Fatalf("Failed to restart rest service: %v", err)
						continue
					}
				}

				quit = make(chan struct{})
				if isVote {
					err = startAllianceService(ctx, wallet, voting, quit) // TODO:restart need update the wallet
					if err != nil {
						log.Fatalf("Failed to start alliance service: %v", err)
					}
				}

				log.Info("The block header is not updated for a long time. Restart the service")
				if isrb {
					again = false
				} else {
					again = true
				}
				timer.Reset(td / 2)
			} else {
				again = false
				timer.Reset(td)
			}
			lasth = sh.Height
		}
	} else {
		waitToExit()
	}
}

func startServer(ctx *cli.Context, wallet *spvwallet.SPVWallet) (restful.ApiServer, error) {
	configPath := ctx.GlobalString(spvwallet.GetFlagName(spvwallet.RestConfigPathFlag))
	servConfig, err := spvwallet.NewRestConfig(configPath)
	if err != nil {
		return nil, err
	}

	serv := service.NewService(wallet, servConfig)
	restServer := restful.InitRestServer(serv, servConfig.Port)
	go restServer.Start()
	//go checkLogFile(logLevel)

	return restServer, nil
}

func startAllianceService(ctx *cli.Context, wallet *spvwallet.SPVWallet, voting chan *btc.BtcProof, quit chan struct{}) error {
	conf, err := alliance.NewAlliaConfig(ctx.GlobalString(spvwallet.GetFlagName(spvwallet.AlliaConfigFile)))
	if err != nil {
		return err
	}

	allia := sdk.NewMultiChainSdk()
	allia.NewRpcClient().SetAddress(conf.AllianceJsonRpcAddress)
	acct, err := alliance.GetAccountByPassword(allia, conf.WalletFile, conf.WalletPwd)
	if err != nil {
		return fmt.Errorf("GetAccountByPassword failed: %v", err)
	}

	ob := alliance.NewObserver(allia, &alliance.ObConfig{
		FirstN:       conf.AlliaObFirstN,
		LoopWaitTime: conf.AlliaObLoopWaitTime,
		WatchingKey:  conf.WatchingKey,
	}, voting, quit)
	go ob.Listen()

	redeem, err := hex.DecodeString(conf.Redeem)
	if err != nil {
		return fmt.Errorf("failed to decode redeem %s: %v", conf.Redeem, err)
	}
	v, err := alliance.NewVoter(allia, voting, wallet, redeem, acct, conf.GasPrice, conf.GasLimit, conf.WaitingDBPath,
		conf.BlksToWait, quit)

	go v.Vote()
	go v.WaitingRetry()

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
	if err := agent.Listen(agent.Options{}); err != nil {
		log.Fatal(err)
	}

	if err := setupApp().Run(os.Args); err != nil {
		log.Errorf("fail to run: %v", err)
		os.Exit(1)
	}
}
