package alliance

import (
	"errors"
	"github.com/boltdb/bolt"
	"github.com/ontio/multi-chain/common"
	"github.com/ontio/multi-chain/native/service/cross_chain_manager/btc"
	"path"
	"strings"
	"sync"
)

var (
	BKTWaiting = []byte("Waiting")
	BKTVoted   = []byte("voted")
)

type WaitingDB struct {
	lock     *sync.RWMutex
	db       *bolt.DB
	filePath string
}

func NewWaitingDB(filePath string) (*WaitingDB, error) {
	if !strings.Contains(filePath, ".bin") {
		filePath = path.Join(filePath, "waiting.bin")
	}
	w := new(WaitingDB)
	db, err := bolt.Open(filePath, 0644, &bolt.Options{InitialMmapSize: 500000})
	if err != nil {
		return nil, err
	}
	w.db = db
	w.lock = new(sync.RWMutex)
	w.filePath = filePath

	if err = db.Update(func(btx *bolt.Tx) error {
		_, err := btx.CreateBucketIfNotExists(BKTWaiting)
		if err != nil {
			return err
		}

		_, err = btx.CreateBucketIfNotExists(BKTVoted)
		if err != nil {
			return err
		}

		return nil
	}); err != nil {
		return nil, err
	}

	return w, nil
}

func (w *WaitingDB) Put(txid []byte, item *btc.BtcProof) error {
	w.lock.Lock()
	defer w.lock.Unlock()

	return w.db.Update(func(btx *bolt.Tx) error {
		bucket := btx.Bucket(BKTWaiting)

		sink := common.NewZeroCopySink(nil)
		item.Serialization(sink)
		val := sink.Bytes()

		err := bucket.Put(txid, val)
		if err != nil {
			return err
		}

		return nil
	})
}

func (w *WaitingDB) Get(txid []byte) (p *btc.BtcProof, err error) {
	w.lock.RLock()
	defer w.lock.RUnlock()

	p = &btc.BtcProof{}
	err = w.db.View(func(tx *bolt.Tx) error {
		bw := tx.Bucket(BKTWaiting)
		val := bw.Get(txid)
		if val == nil {
			return errors.New("not found in db")
		}

		source := common.NewZeroCopySource(val)
		err = p.Deserialization(source)
		if err != nil {
			return err
		}
		return nil
	})

	return
}

func (w *WaitingDB) GetUnderHeightAndDelete(height uint32) ([]*btc.BtcProof, [][]byte, error) {
	w.lock.Lock()
	defer w.lock.Unlock()

	arr := make([]*btc.BtcProof, 0)
	keys := make([][]byte, 0)
	err := w.db.Update(func(tx *bolt.Tx) error {
		bw := tx.Bucket(BKTWaiting)
		err := bw.ForEach(func(k, v []byte) error {
			p := &btc.BtcProof{}
			err := p.Deserialization(common.NewZeroCopySource(v))
			if err != nil {
				return err
			}
			if p.Height <= height {
				key := make([]byte, len(k))
				copy(key, k)
				arr = append(arr, p)
				keys = append(keys, key)
			}
			return nil
		})
		if err != nil {
			return err
		}

		for _, k := range keys {
			err = bw.Delete(k)
			if err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, nil, err
	}

	return arr, keys, nil
}

func (w *WaitingDB) MarkVotedTx(txid []byte) error {
	w.lock.Lock()
	defer w.lock.Unlock()

	return w.db.Update(func(btx *bolt.Tx) error {
		bucket := btx.Bucket(BKTVoted)
		err := bucket.Put(txid, []byte{1})
		if err != nil {
			return err
		}
		return nil
	})
}

func (w *WaitingDB) CheckIfVoted(txid []byte) bool {
	w.lock.RLock()
	defer w.lock.RUnlock()

	exist := false
	_ = w.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(BKTVoted)
		if bucket.Get(txid) != nil {
			exist = true
		}
		return nil
	})

	return exist
}

func (w *WaitingDB) CheckIfWaiting(txid []byte) bool {
	w.lock.RLock()
	defer w.lock.RUnlock()

	exist := false
	_ = w.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(BKTWaiting)
		if bucket.Get(txid) != nil {
			exist = true
		}
		return nil
	})

	return exist
}

func (w *WaitingDB) DelIfExist(txid []byte) bool {
	w.lock.Lock()
	defer w.lock.Unlock()

	exist := false
	_ = w.db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(BKTWaiting)
		if bucket.Get(txid) != nil {
			err := bucket.Delete(txid)
			if err == nil {
				exist = true
			}
		}
		return nil
	})

	return exist
}

func (w *WaitingDB) Close() {
	w.lock.Lock()
	w.db.Close()
	w.lock.Unlock()
}
