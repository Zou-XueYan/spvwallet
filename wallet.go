package spvclient

import (
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/wire"
	btc "github.com/btcsuite/btcutil"
	hd "github.com/btcsuite/btcutil/hdkeychain"
	"github.com/btcsuite/btcwallet/wallet/txrules"
	"github.com/ontio/spvclient/chain"
	"github.com/ontio/spvclient/log"
	"github.com/ontio/spvclient/netserv"
	"time"
)

type SPVWallet struct {
	params      *chaincfg.Params
	repoPath    string
	Blockchain  *chain.Blockchain
	peerManager *netserv.PeerManager
	wireService *netserv.WireService
	running     bool
	config      *netserv.PeerManagerConfig
}

const WALLET_VERSION = "0.1.0"

func NewSPVWallet(config *Config) (*SPVWallet, error) {
	w := &SPVWallet{
		repoPath: config.RepoPath,
		params:   config.Params,
	}

	var err error
	if err != nil {
		return nil, err
	}

	w.Blockchain, err = chain.NewBlockchain(w.repoPath, w.params, config.IsVote)
	if err != nil {
		return nil, err
	}
	minSync := 5
	if config.TrustedPeer != nil {
		minSync = 1
	}
	wireConfig := &netserv.WireServiceConfig{
		Chain:           w.Blockchain,
		MinPeersForSync: minSync,
		Params:          w.params,
	}

	ws := netserv.NewWireService(wireConfig)
	w.wireService = ws

	getNewestBlock := func() (*chainhash.Hash, int32, error) {
		sh, err := w.Blockchain.BestBlock()
		if err != nil {
			return nil, 0, err
		}
		h := sh.Header.BlockHash()
		return &h, int32(sh.Height), nil
	}

	w.config = &netserv.PeerManagerConfig{
		UserAgentName:    config.UserAgent,
		UserAgentVersion: WALLET_VERSION,
		Params:           w.params,
		AddressCacheDir:  config.RepoPath,
		Proxy:            config.Proxy,
		GetNewestBlock:   getNewestBlock,
		MsgChan:          ws.MsgChan(),
	}

	if config.TrustedPeer != nil {
		w.config.TrustedPeer = config.TrustedPeer
	}

	w.peerManager, err = netserv.NewPeerManager(w.config)
	if err != nil {
		return nil, err
	}

	return w, nil
}

func (w *SPVWallet) Start() {
	w.running = true
	go w.wireService.Start()
	go w.peerManager.Start()
}

//////////////////////////////////////////////////////////////////////////////////////////////////////////////////
//
// API
//
//////////////

func (w *SPVWallet) CurrencyCode() string {
	if w.params.Name == chaincfg.MainNetParams.Name {
		return "btc"
	} else {
		return "tbtc"
	}
}

func (w *SPVWallet) IsDust(amount int64) bool {
	return txrules.IsDustAmount(btc.Amount(amount), 25, txrules.DefaultRelayFeePerKb)
}

func (w *SPVWallet) ChildKey(keyBytes []byte, chaincode []byte, isPrivateKey bool) (*hd.ExtendedKey, error) {
	parentFP := []byte{0x00, 0x00, 0x00, 0x00}
	var id []byte
	if isPrivateKey {
		id = w.params.HDPrivateKeyID[:]
	} else {
		id = w.params.HDPublicKeyID[:]
	}
	hdKey := hd.NewExtendedKey(
		id,
		keyBytes,
		chaincode,
		parentFP,
		0,
		0,
		isPrivateKey)
	return hdKey.Child(0)
}

func (w *SPVWallet) Params() *chaincfg.Params {
	return w.params
}

func (w *SPVWallet) ChainTip() (uint32, chainhash.Hash) {
	var ch chainhash.Hash
	sh, err := w.Blockchain.BestBlock()
	if err != nil {
		return 0, ch
	}
	return sh.Height, sh.Header.BlockHash()
}

func (w *SPVWallet) Close() {
	if w.running {
		log.Info("Disconnecting from peers and shutting down")
		w.peerManager.Stop()
		w.Blockchain.Close()
		w.wireService.Stop()
		w.running = false
	}
}

func (w *SPVWallet) ReSyncBlockchain(fromDate time.Time) {
	w.Blockchain.Rollback(fromDate)
	//w.txstore.PopulateAdrs()
	w.wireService.Resync()
}

func (w *SPVWallet) ReSync() {
	w.wireService.ResyncWithNil()
}

func (s *SPVWallet) Broadcast(tx *wire.MsgTx) error {
	log.Debugf("Broadcasting tx %s to peers", tx.TxHash().String())
	for _, p := range s.peerManager.ConnectedPeers() {
		p.QueueMessageWithEncoding(tx, nil, wire.LatestEncoding)
	}
	return nil
}
