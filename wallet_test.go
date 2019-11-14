package spvwallet

import (
	"github.com/ontio/spvclient/db"
	"github.com/ontio/spvclient/log"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcutil"
	"os"
	"testing"
	"time"
)

func TestSPVWallet_Restart(t *testing.T) {
	log.InitLog(0, log.Stdout)

	conf := NewDefaultConfig()
	conf.RepoPath = "./"
	conf.Params = &chaincfg.TestNet3Params

	sqliteDatastore, err := db.Create(conf.RepoPath)
	if err != nil {
		log.Fatalf("Failed to create db: %v", err)
		return
	}
	conf.DB = sqliteDatastore

	wa, _ := btcutil.DecodeAddress("2N5cY8y9RtbbvQRWkX5zAwTPCxSZF9xEj2C", &chaincfg.TestNet3Params)
	wallet, _ := NewSPVWallet(conf)
	wallet.AddWatchedAddress(wa)

	wallet.Start()

	time.Sleep(2 * time.Second)
	wallet.wireService.syncPeer = nil
	//wallet.Close()
	//wallet, err = NewSPVWallet(conf)
	//if err != nil {
	//	t.Fatalf("Failed to new a spvwallet: %v", err)
	//}
	//wallet.AddWatchedAddress(wa)
	//wallet.Start()
	//log.Info("The block header is not updated for a long time. Restart the service")
	log.Info("-----------------------resync-----------------------")
	wallet.wireService.Resync()
	time.Sleep(10 * time.Second)
	wallet.Close()

	os.RemoveAll("headers.bin")
	os.RemoveAll("wallet.db")
	os.RemoveAll("peers.json")
}
