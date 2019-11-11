package main

import (
	"encoding/hex"
	"fmt"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/google/gops/agent"
	sdk "github.com/ontio/multi-chain-go-sdk"
	"github.com/ontio/multi-chain/native/service/cross_chain_manager/btc"
	"github.com/ontio/spvwallet"
	"github.com/ontio/spvwallet/alliance"
	"github.com/ontio/spvwallet/config"
	"github.com/ontio/spvwallet/log"
	"github.com/ontio/spvwallet/rest/http/restful"
	"github.com/ontio/spvwallet/rest/service"
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
	app.Action = run
	app.Copyright = ""
	app.Flags = []cli.Flag{
		spvwallet.LogLevelFlag,
		spvwallet.ConfigFile,
		spvwallet.GoMaxProcs,
	}
	app.Before = func(context *cli.Context) error {
		cores := context.GlobalInt(spvwallet.GoMaxProcs.Name)
		runtime.GOMAXPROCS(cores)
		return nil
	}
	return app
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

func run(ctx *cli.Context) {
	logLevel := ctx.GlobalInt(spvwallet.GetFlagName(spvwallet.LogLevelFlag))
	log.InitLog(logLevel, log.Stdout)

	conf, err := config.NewConfig(ctx.GlobalString(spvwallet.GetFlagName(spvwallet.ConfigFile)))
	if err != nil {
		log.Errorf("failed to new a config: %v", err)
		os.Exit(1)
	}
	if conf.SleepTime > 0 {
		config.SleepTime = time.Duration(conf.SleepTime)
	}

	var netType *chaincfg.Params
	switch conf.ConfigBitcoinNet {
	case "regtest":
		netType = &chaincfg.RegressionNetParams
	case "test":
		netType = &chaincfg.TestNet3Params
	case "sim":
		netType = &chaincfg.SimNetParams
	default:
		log.Errorf("wrong net type: %s", conf.ConfigBitcoinNet)
		os.Exit(1)
	}

	wallet, err := startSpv(conf, netType)
	if err != nil {
		log.Errorf("failed to start spv: %v", err)
		os.Exit(1)
	}
	if conf.RunRest == 1 {
		_, err = startServer(conf, wallet)
		if err != nil {
			log.Fatalf("Failed to start rest service: %v", err)
			os.Exit(1)
		}
	}

	voting := make(chan *btc.BtcProof, 10)
	txchan := make(chan *alliance.ToSignItem, 10)
	if conf.RunVote == 1 {
		_, _, err = startAllianceService(conf, wallet, voting, txchan, netType)
		if err != nil {
			log.Fatalf("Failed to start alliance service: %v", err)
		}
	}

	if conf.IsRestart == 1 {
		resyncSpv(wallet, conf.RestartDuration)
	} else {
		waitToExit()
	}
}

func startSpv(c *config.Config, netType *chaincfg.Params) (*spvwallet.SPVWallet, error) {
	conf := spvwallet.NewDefaultConfig()
	conf.IsVote = c.RunVote == 1

	if c.ConfigDBPath != "" {
		conf.RepoPath = c.ConfigDBPath
	}

	conf.Params = netType
	if netType.Name == "regtest" {
		conf.RepoPath = path.Join(conf.RepoPath, "regtest")
	}
	if c.TrustedPeer != "" {
		conf.TrustedPeer, _ = net.ResolveTCPAddr("tcp", c.TrustedPeer+":"+conf.Params.DefaultPort)
	}

	wallet, err := spvwallet.NewSPVWallet(conf)
	if err != nil {
		return nil, err
	}
	wallet.Start()

	return wallet, nil
}

func startServer(conf *config.Config, wallet *spvwallet.SPVWallet) (restful.ApiServer, error) {
	serv := service.NewService(wallet)
	restServer := restful.InitRestServer(serv, conf.RestPort)
	go restServer.Start()

	return restServer, nil
}

func startAllianceService(conf *config.Config, wallet *spvwallet.SPVWallet, voting chan *btc.BtcProof,
	txchan chan *alliance.ToSignItem, params *chaincfg.Params) (*alliance.Observer, *alliance.Voter, error) {
	allia := sdk.NewMultiChainSdk()
	allia.NewRpcClient().SetAddress(conf.AllianceJsonRpcAddress)
	acct, err := alliance.GetAccountByPassword(allia, conf.WalletFile, conf.WalletPwd)
	if err != nil {
		return nil, nil, fmt.Errorf("GetAccountByPassword failed: %v", err)
	}

	ob := alliance.NewObserver(allia, voting, txchan, conf.AlliaObLoopWaitTime, conf.WatchingKey, conf.WatchingMakeTxKey,
		conf.AlliaNet)
	go ob.Listen()

	redeem, err := hex.DecodeString(conf.Redeem)
	if err != nil {
		return ob, nil, fmt.Errorf("failed to decode redeem %s: %v", conf.Redeem, err)
	}
	v, err := alliance.NewVoter(allia, voting, wallet, redeem, acct, conf.WaitingDBPath,
		conf.BlksToWait)
	if err != nil {
		return ob, v, fmt.Errorf("failed to new a voter: %v", err)
	}

	go v.Vote()
	go v.WaitingRetry()

	signer, err := alliance.NewSigner(conf.BtcPrivk, txchan, acct, allia, params)
	if err != nil {
		return ob, v, fmt.Errorf("failed to new a signer: %v", err)
	}
	go signer.Signing()

	return ob, v, nil
}

func resyncSpv(wallet *spvwallet.SPVWallet, dura int) {
	sh, err := wallet.Blockchain.BestBlock()
	if err != nil {
		log.Fatalf("Failed to get best block: %v", err)
		os.Exit(1)
	}
	lasth := sh.Height

	again := false
	td := time.Duration(dura) * time.Minute
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
			log.Debugf("Restart now")
			//if isVote {
			//	log.Debugf("stop voter")
			//	voter.Stop()
			//}
			//if restServer != nil {
			//	log.Debugf("stop rest service")
			//	restServer.Stop()
			//}

			if again {
				log.Debugf("It happened TWICE")
				err = wallet.Blockchain.Rollback(sh.Header.Timestamp.Add(-6 * time.Hour))
				if err != nil {
					log.Fatalf("Failed to rollback: %v", err)
					continue
				}
				isrb = true
				//_ = os.RemoveAll(path.Join(config.RepoPath, "peers.json"))
			}

			wallet.ReSync()
			//wallet.Close()
			//
			//wallet, _ = spvwallet.NewSPVWallet(config)
			//wallet.Start()

			//if ctx.GlobalInt(spvwallet.RunRest.Name) == 1 {
			//	restServer, err = startServer(ctx, wallet)
			//	if err != nil {
			//		log.Fatalf("Failed to restart rest service: %v", err)
			//		continue
			//	}
			//}

			//if isVote {
			//	voter.Restart(wallet)
			//}

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
