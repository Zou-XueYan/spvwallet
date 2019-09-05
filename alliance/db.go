package alliance

import (
	"errors"
	"github.com/boltdb/bolt"
	"github.com/ontio/multi-chain/common"
	"github.com/ontio/multi-chain/smartcontract/service/native/cross_chain_manager/btc"
	"path"
	"strings"
	"sync"
)

var (
	BKTWaiting = []byte("Waiting")
	BKTVoted   = []byte("voted")
)

type WatingDB struct {
	lock     *sync.Mutex
	db       *bolt.DB
	filePath string
}

func NewWaitingDB(filePath string) (*WatingDB, error) {
	if !strings.Contains(filePath, ".bin") {
		filePath = path.Join(filePath, "waiting.bin")
	}
	w := new(WatingDB)
	db, err := bolt.Open(filePath, 0644, &bolt.Options{InitialMmapSize: 500000})
	if err != nil {
		return nil, err
	}
	w.db = db
	w.lock = new(sync.Mutex)
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

func (w *WatingDB) Put(txid []byte, item *btc.BtcProof) error {
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

func (w *WatingDB) Get(txid []byte) (p *btc.BtcProof, err error) {
	w.lock.Lock()
	defer w.lock.Unlock()

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

func (w *WatingDB) GetUnderHeightAndDelte(height uint32) ([]*btc.BtcProof, [][]byte, error) {
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
				arr = append(arr, p)
				keys = append(keys, k)
			}
			return nil
		})

		for _, k := range keys {
			err = bw.Delete(k)
			if err != nil {
				return err
			}
		}
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return nil, nil, err
	}

	return arr, keys, nil
}

func (w *WatingDB) MarkVotedTx(txid []byte) error {
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

func (w *WatingDB) CheckIfVoted(txid []byte) bool {
	w.lock.Lock()
	defer w.lock.Unlock()

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

func (w *WatingDB) Close() {
	w.lock.Lock()
	w.db.Close()
}
