// Copyright (C) 2015-2016 The Lightning Network Developers
// Copyright (c) 2016-2017 The OpenBazaar Developers

package chain

import (
	"errors"
	"github.com/btcsuite/btcd/blockchain"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/wire"
	"github.com/ontio/spvwallet/log"
	"math/big"
	"sync"
	"time"
)

// Blockchain settings.  These are kindof Bitcoin specific, but not contained in
// chaincfg.Params so they'll go here.  If you're into the [ANN]altcoin scene,
// you may want to paramaterize these constants.
const (
	targetTimespan      = time.Hour * 24 * 14
	targetSpacing       = time.Minute * 10
	epochLength         = int32(targetTimespan / targetSpacing) // 2016
	maxDiffAdjust       = 4
	minRetargetTimespan = int64(targetTimespan / maxDiffAdjust)
	maxRetargetTimespan = int64(targetTimespan * maxDiffAdjust)
	medianTimeBlocks    = 11
)

var OrphanHeaderError = errors.New("header does not extend any known headers")

// Wrapper around Headers implementation that handles all blockchain operations
type Blockchain struct {
	lock         *sync.Mutex
	params       *chaincfg.Params
	db           Headers
	HeaderUpdate chan uint32
	IsOpen       bool
}

func NewBlockchain(filePath string, params *chaincfg.Params, isOpen bool) (*Blockchain, error) {
	hdb, err := NewHeaderDB(filePath)
	if err != nil {
		return nil, err
	}
	b := &Blockchain{
		lock:         new(sync.Mutex),
		params:       params,
		db:           hdb,
		HeaderUpdate: make(chan uint32, 100),
		IsOpen:       isOpen,
	}

	h, err := b.db.Height()
	if h == 0 || err != nil {
		log.Info("Initializing headers db with checkpoints")
		checkpoint := GetCheckpoint(time.Now(), params)
		// Put the checkpoint to the db
		sh := StoredHeader{
			Header:    checkpoint.Header,
			Height:    checkpoint.Height,
			totalWork: big.NewInt(0),
		}
		err := b.db.Put(sh, true)
		if err != nil {
			return nil, err
		}
	}
	return b, nil
}

func (b *Blockchain) CommitHeader(header wire.BlockHeader) (bool, *StoredHeader, uint32, error) {
	b.lock.Lock()
	defer b.lock.Unlock()
	newTip := false
	var commonAncestor *StoredHeader
	// Fetch our current best header from the db
	bestHeader, err := b.db.GetBestHeader()
	if err != nil {
		return false, nil, 0, err
	}
	tipHash := bestHeader.Header.BlockHash()
	var parentHeader StoredHeader

	// If the tip is also the parent of this header, then we can save a database read by skipping
	// the lookup of the parent header. Otherwise (ophan?) we need to fetch the parent.
	if header.PrevBlock.IsEqual(&tipHash) {
		parentHeader = bestHeader
	} else {
		parentHeader, err = b.db.GetPreviousHeader(header)
		if err != nil {
			return false, nil, 0, OrphanHeaderError
		}
	}
	valid := b.CheckHeader(header, parentHeader)
	if !valid {
		return false, nil, 0, nil
	}
	// If this block is already the tip, return
	headerHash := header.BlockHash()
	if tipHash.IsEqual(&headerHash) {
		return newTip, nil, 0, nil
	}
	// Add the work of this header to the total work stored at the previous header
	cumulativeWork := new(big.Int).Add(parentHeader.totalWork, blockchain.CalcWork(header.Bits))

	// If the cumulative work is greater than the total work of our best header
	// then we have a new best header. Update the chain tip and check for a reorg.
	if cumulativeWork.Cmp(bestHeader.totalWork) == 1 {
		newTip = true
		prevHash := parentHeader.Header.BlockHash()
		// If this header is not extending the previous best header then we have a reorg.
		if !tipHash.IsEqual(&prevHash) {
			commonAncestor, err = b.GetCommonAncestor(StoredHeader{Header: header, Height: parentHeader.Height + 1}, bestHeader)
			if err != nil {
				log.Errorf("Error calculating common ancestor: %s", err.Error())
				return newTip, commonAncestor, 0, err
			}

			if chash := commonAncestor.Header.BlockHash(); chash.IsEqual(&tipHash) {
				commonAncestor = nil
				log.Warnf("commonAncestor is our best %s, so make the new header %s our best", tipHash.String(), headerHash.String())
			} else {
				log.Warnf("REORG!!! REORG!!! REORG!!! At block %d, Wiped out %d blocks", int(bestHeader.Height), int(bestHeader.Height-commonAncestor.Height))
			}
		}
	}

	newHeight := parentHeader.Height + 1
	// Put the header to the database
	err = b.db.Put(StoredHeader{
		Header:    header,
		Height:    newHeight,
		totalWork: cumulativeWork,
	}, newTip)
	if err != nil {
		return newTip, commonAncestor, 0, err
	}

	if b.IsOpen && newTip {
		b.HeaderUpdate <- newHeight
	}
	return newTip, commonAncestor, newHeight, nil
}

func (b *Blockchain) CheckHeader(header wire.BlockHeader, prevHeader StoredHeader) bool {
	// Get hash of n-1 header
	prevHash := prevHeader.Header.BlockHash()
	height := prevHeader.Height

	// Check if headers link together.  That whole 'blockchain' thing.
	if prevHash.IsEqual(&header.PrevBlock) == false {
		log.Errorf("Headers %d and %d don't link.\n", height, height+1)
		return false
	}

	// Check the header meets the difficulty requirement
	if !b.params.ReduceMinDifficulty {
		diffTarget, err := b.calcRequiredWork(header, int32(height+1), prevHeader)
		if err != nil {
			log.Errorf("Error calclating difficulty", err)
			return false
		}
		if header.Bits != diffTarget {
			log.Warnf("Block %d %s incorrect difficulty.  Read %d, expect %d\n",
				height+1, header.BlockHash().String(), header.Bits, diffTarget)
			return false
		}
	}

	// Check if there's a valid proof of work.  That whole "Bitcoin" thing.
	if !checkProofOfWork(header, b.params) {
		log.Debugf("Block %d bad proof of work.\n", height+1)
		return false
	}

	return true // it must have worked if there's no errors and got to the end.
}

// Get the PoW target this block should meet. We may need to handle a difficulty adjustment
// or testnet difficulty rules.
func (b *Blockchain) calcRequiredWork(header wire.BlockHeader, height int32, prevHeader StoredHeader) (uint32, error) {
	// If this is not a difficulty adjustment period
	if height%epochLength != 0 {
		// If we are on testnet
		if b.params.ReduceMinDifficulty {
			// If it's been more than 20 minutes since the last header return the minimum difficulty
			if header.Timestamp.After(prevHeader.Header.Timestamp.Add(targetSpacing * 2)) {
				return b.params.PowLimitBits, nil
			} else { // Otherwise return the difficulty of the last block not using special difficulty rules
				for {
					var err error = nil
					for err == nil && int32(prevHeader.Height)%epochLength != 0 && prevHeader.Header.Bits == b.params.PowLimitBits {
						var sh StoredHeader
						sh, err = b.db.GetPreviousHeader(prevHeader.Header)
						// Error should only be non-nil if prevHeader is the checkpoint.
						// In that case we should just return checkpoint bits
						if err == nil {
							prevHeader = sh
						}

					}
					return prevHeader.Header.Bits, nil
				}
			}
		}
		// Just return the bits from the last header
		return prevHeader.Header.Bits, nil
	}
	// We are on a difficulty adjustment period so we need to correctly calculate the new difficulty.
	epoch, err := b.GetEpoch()
	if err != nil {
		log.Error(err)
		return 0, err
	}
	return calcDiffAdjust(*epoch, prevHeader.Header, b.params), nil
}

func (b *Blockchain) GetEpoch() (*wire.BlockHeader, error) {
	sh, err := b.db.GetBestHeader()
	if err != nil {
		return &sh.Header, err
	}
	for i := 0; i < 2015; i++ {
		sh, err = b.db.GetPreviousHeader(sh.Header)
		if err != nil {
			return &sh.Header, err
		}
	}
	log.Debug("Epoch", sh.Header.BlockHash().String())
	return &sh.Header, nil
}

func (b *Blockchain) GetNPrevBlockHashes(n int) []*chainhash.Hash {
	var ret []*chainhash.Hash
	hdr, err := b.db.GetBestHeader()
	if err != nil {
		return ret
	}
	tipSha := hdr.Header.BlockHash()
	ret = append(ret, &tipSha)
	for i := 0; i < n-1; i++ {
		hdr, err = b.db.GetPreviousHeader(hdr.Header)
		if err != nil {
			return ret
		}
		shaHash := hdr.Header.BlockHash()
		ret = append(ret, &shaHash)
	}
	return ret
}

func (b *Blockchain) GetBlockLocator() blockchain.BlockLocator {
	var ret []*chainhash.Hash
	parent, err := b.db.GetBestHeader()
	if err != nil {
		return ret
	}

	rollback := func(parent StoredHeader, n int) (StoredHeader, error) {
		for i := 0; i < n; i++ {
			parent, err = b.db.GetPreviousHeader(parent.Header)
			if err != nil {
				return parent, err
			}
		}
		return parent, nil
	}

	step := 1
	start := 0
	for {
		if start >= 9 {
			step *= 2
			start = 0
		}
		hash := parent.Header.BlockHash()
		ret = append(ret, &hash)
		if len(ret) == 500 {
			break
		}
		parent, err = rollback(parent, step)
		if err != nil {
			break
		}
		start += 1
	}
	return blockchain.BlockLocator(ret)
}

// GetCommonAncestor returns last header before reorg point
func (b *Blockchain) GetCommonAncestor(bestHeader, prevBestHeader StoredHeader) (*StoredHeader, error) {
	var err error
	rollback := func(parent StoredHeader, n int) (StoredHeader, error) {
		for i := 0; i < n; i++ {
			parent, err = b.db.GetPreviousHeader(parent.Header)
			if err != nil {
				return parent, err
			}
		}
		return parent, nil
	}

	majority := bestHeader
	minority := prevBestHeader
	if bestHeader.Height > prevBestHeader.Height {
		majority, err = rollback(majority, int(bestHeader.Height-prevBestHeader.Height))
		if err != nil {
			return nil, err
		}
	} else if prevBestHeader.Height > bestHeader.Height {
		minority, err = rollback(minority, int(prevBestHeader.Height-bestHeader.Height))
		if err != nil {
			return nil, err
		}
	}

	for {
		majorityHash := majority.Header.BlockHash()
		minorityHash := minority.Header.BlockHash()
		if majorityHash.IsEqual(&minorityHash) {
			return &majority, nil
		}
		majority, err = b.db.GetPreviousHeader(majority.Header)
		if err != nil {
			return nil, err
		}
		minority, err = b.db.GetPreviousHeader(minority.Header)
		if err != nil {
			return nil, err
		}
	}
}

// Rollback the header database to the last header before time t.
// We shouldn't go back further than the checkpoint
func (b *Blockchain) Rollback(t time.Time) error {
	b.lock.Lock()
	defer b.lock.Unlock()
	checkpoint := GetCheckpoint(time.Now(), b.params)
	checkPointHash := checkpoint.Header.BlockHash()
	sh, err := b.db.GetBestHeader()
	if err != nil {
		return err
	}
	// If t is greater than the timestamp at the tip then do nothing
	if sh.Header.Timestamp.Before(t) {
		return nil
	}
	// If the tip is our checkpoint then do nothing
	checkHash := sh.Header.BlockHash()
	if checkHash.IsEqual(&checkPointHash) {
		return nil
	}
	rollbackHeight := uint32(0)
	for i := 0; i < 1000000000; i++ {
		sh, err = b.db.GetPreviousHeader(sh.Header)
		if err != nil {
			return err
		}
		checkHash := sh.Header.BlockHash()
		// If we rolled back to the checkpoint then stop here and set the checkpoint as the tip
		if checkHash.IsEqual(&checkPointHash) {
			rollbackHeight = checkpoint.Height
			break
		}
		// If we hit a header created before t then stop here and set this header as the tip
		if sh.Header.Timestamp.Before(t) {
			rollbackHeight = sh.Height
			break
		}
	}
	err = b.db.DeleteAfter(rollbackHeight)
	if err != nil {
		return err
	}
	return b.db.Put(sh, true)
}

func (b *Blockchain) BestBlock() (StoredHeader, error) {
	sh, err := b.db.GetBestHeader()
	if err != nil {
		return StoredHeader{}, err
	}
	return sh, nil
}

// add by zou
func (b *Blockchain) GetHeaderByHeight(height uint32) (sh StoredHeader, err error) {
	sh, err = b.db.GetHeaderByHeight(height)
	if err != nil {
		return sh, err
	}
	return sh, nil
}

func (b *Blockchain) GetHeader(hash *chainhash.Hash) (StoredHeader, error) {
	sh, err := b.db.GetHeader(*hash)
	if err != nil {
		return sh, err
	}
	return sh, nil
}

func (b *Blockchain) Close() {
	b.lock.Lock()
	b.db.Close()
	b.lock.Unlock()
}

// Verifies the header hashes into something lower than specified by the 4-byte bits field.
func checkProofOfWork(header wire.BlockHeader, p *chaincfg.Params) bool {
	target := blockchain.CompactToBig(header.Bits)

	// The target must more than 0.  Why can you even encode negative...
	if target.Sign() <= 0 {
		log.Debugf("Block target %064x is neagtive(??)\n", target.Bytes())
		return false
	}
	// The target must be less than the maximum allowed (difficulty 1)
	if target.Cmp(p.PowLimit) > 0 {
		log.Debugf("Block target %064x is "+
			"higher than max of %064x", target, p.PowLimit.Bytes())
		return false
	}
	// The header hash must be less than the claimed target in the header.
	blockHash := header.BlockHash()
	hashNum := blockchain.HashToBig(&blockHash)
	if hashNum.Cmp(target) > 0 {
		log.Debugf("Block hash %064x is higher than "+
			"required target of %064x", hashNum, target)
		return false
	}
	return true
}

// This function takes in a start and end block header and uses the timestamps in each
// to calculate how much of a difficulty adjustment is needed. It returns a new compact
// difficulty target.
func calcDiffAdjust(start, end wire.BlockHeader, p *chaincfg.Params) uint32 {
	duration := end.Timestamp.UnixNano() - start.Timestamp.UnixNano()
	if duration < minRetargetTimespan {
		log.Debugf("Whoa there, block %s off-scale high 4X diff adjustment!",
			end.BlockHash().String())
		duration = minRetargetTimespan
	} else if duration > maxRetargetTimespan {
		log.Debugf("Uh-oh! block %s off-scale low 0.25X diff adjustment!\n",
			end.BlockHash().String())
		duration = maxRetargetTimespan
	}

	// calculation of new 32-byte difficulty target
	// first turn the previous target into a big int
	prevTarget := blockchain.CompactToBig(end.Bits)
	// new target is old * duration...
	newTarget := new(big.Int).Mul(prevTarget, big.NewInt(duration))
	// divided by 2 weeks
	newTarget.Div(newTarget, big.NewInt(int64(targetTimespan)))

	// clip again if above minimum target (too easy)
	if newTarget.Cmp(p.PowLimit) > 0 {
		newTarget.Set(p.PowLimit)
	}

	return blockchain.BigToCompact(newTarget)
}
