package blockchain

import (
	"bytes"
	"crypto/sha256"
	"encoding/gob"
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"runtime"

	"github.com/dgraph-io/badger"
	"github.com/sirupsen/logrus"
)

const (
	DB_PATH      = "./tmp/blocks"
	DB_FILE      = "./tmp/blocks/MANIFEST"
	GENESIS_DATA = "First Transaction from Genesis"
)

type Block struct {
	Hash         []byte
	Transactions []*Transaction
	PrevHash     []byte
	Nonce        int
}

type BlockChain struct {
	LastHash []byte
	Database *badger.DB
}

type BlockChainIterator struct {
	CurrentHash []byte
	Database    *badger.DB
}

func CreateBlock(txs []*Transaction, previusHash []byte) *Block {
	block := &Block{[]byte{}, txs, previusHash, 0}
	pow := NewProof(block)
	nonce, hash := pow.Run()
	block.Hash = hash
	block.Nonce = nonce
	return block
}

func DBexist() bool {
	if _, err := os.Stat(DB_FILE); os.IsNotExist(err) {
		return false
	}

	return true
}

func (chain *BlockChain) AddBlock(txs []*Transaction) {
	var lastHash []byte

	err := chain.Database.View(func(txn *badger.Txn) error {
		item, err := txn.Get([]byte("lh"))
		Handle(err)
		err = item.Value(func(val []byte) error {
			lastHash = val
			return nil
		})
		return err
	})
	Handle(err)

	newBlock := CreateBlock(txs, lastHash)

	err = chain.Database.Update(func(txn *badger.Txn) error {
		err := txn.Set(newBlock.Hash, newBlock.Serialize())
		Handle(err)
		err = txn.Set([]byte("lh"), newBlock.Hash)
		chain.LastHash = newBlock.Hash
		return err
	})
	Handle(err)

}

func (chain *BlockChain) Iterator() *BlockChainIterator {
	iter := &BlockChainIterator{chain.LastHash, chain.Database}
	return iter
}

func (iter *BlockChainIterator) Next() *Block {
	var block *Block
	err := iter.Database.View(func(txn *badger.Txn) error {
		item, err := txn.Get(iter.CurrentHash)
		var encodedBlock []byte
		err = item.Value(func(val []byte) error {
			encodedBlock = val
			return err
		})
		block = Deserialize(encodedBlock)
		return err
	})
	Handle(err)

	iter.CurrentHash = block.PrevHash

	return block
}

func (b *Block) Serialize() []byte {
	var res bytes.Buffer

	encoder := gob.NewEncoder(&res)
	err := encoder.Encode(b)
	Handle(err)
	return res.Bytes()
}

func (b *Block) HashTransaction() []byte {
	var txHashes [][]byte
	var txHash [32]byte

	for _, tx := range b.Transactions {
		txHashes = append(txHashes, tx.ID)
	}

	txHash = sha256.Sum256(bytes.Join(txHashes, []byte{}))

	return txHash[:]
}

func Deserialize(data []byte) *Block {
	var block Block

	decoder := gob.NewDecoder(bytes.NewReader(data))
	err := decoder.Decode(&block)

	Handle(err)

	return &block
}

func Genesis(coinbase *Transaction) *Block {
	return CreateBlock([]*Transaction{coinbase}, []byte{})
}

func InitBlockChain(adress string) *BlockChain {
	var lastHash []byte

	if DBexist() {
		fmt.Println("Blockchain already exists")
		runtime.Goexit()
	}

	logrus.SetLevel(logrus.ErrorLevel)
	opts := badger.DefaultOptions(DB_PATH)
	opts.Dir = DB_PATH
	opts.Logger = logrus.StandardLogger()
	opts.ValueDir = DB_PATH

	db, err := badger.Open(opts)
	Handle(err)

	err = db.Update(func(txn *badger.Txn) error {
		cbtx := CoinbaseTx(adress, GENESIS_DATA)
		genesis := Genesis(cbtx)
		fmt.Println("Genesis proved")
		err := txn.Set(genesis.Hash, genesis.Serialize())
		Handle(err)
		err = txn.Set([]byte("lh"), genesis.Hash)

		lastHash = genesis.Hash

		return err
	})
	Handle(err)

	blockchain := BlockChain{lastHash, db}
	return &blockchain
}

func ContinueBlockChain(address string) *BlockChain {
	if DBexist() == false {
		fmt.Println("No existing blockchain found, create one!")
		runtime.Goexit()
	}

	var lastHash []byte

	logrus.SetLevel(logrus.ErrorLevel)
	opts := badger.DefaultOptions(DB_PATH)
	opts.Dir = DB_PATH
	opts.Logger = logrus.StandardLogger()
	opts.ValueDir = DB_PATH

	db, err := badger.Open(opts)
	Handle(err)

	err = db.Update(func(txn *badger.Txn) error {
		item, err := txn.Get([]byte("lh"))
		Handle(err)
		err = item.Value(func(val []byte) error {
			lastHash = val
			return nil
		})
		return err
	})

	Handle(err)

	chain := BlockChain{LastHash: lastHash, Database: db}
	return &chain

}

func (chain *BlockChain) FindUnspentTransactions(address string) []Transaction {
	var unspentTxs []Transaction

	spentTXOs := make(map[string][]int)

	iter := chain.Iterator()

	for {
		block := iter.Next()

		for _, tx := range block.Transactions {
			txID := hex.EncodeToString(tx.ID)

		Outputs:
			for outIdx, out := range tx.Outputs {
				if spentTXOs[txID] != nil {
					for _, spentOut := range spentTXOs[txID] {
						if spentOut == outIdx {
							continue Outputs
						}
					}
				}

				if out.CanBeUnlocked(address) {
					unspentTxs = append(unspentTxs, *tx)
				}
			}

		}

		if len(block.PrevHash) == 0 {
			break
		}
	}
	return unspentTxs

}

func (chain *BlockChain) FindUTXO(address string) []TxOutput {
	var UTXOs []TxOutput
	unspentTransaction := chain.FindUnspentTransactions(address)

	for _, tx := range unspentTransaction {
		for _, out := range tx.Outputs {
			if out.CanBeUnlocked(address) {
				UTXOs = append(UTXOs, out)
			}
		}
	}

	return UTXOs

}

func (chain *BlockChain) FindSpendableOutputs(address string, amount int) (int, map[string][]int) {
	unspentOuts := make(map[string][]int)
	unspentTxs := chain.FindUnspentTransactions(address)
	accumulated := 0

Work:
	for _, tx := range unspentTxs {
		TxID := hex.EncodeToString(tx.ID)

		for outIdx, out := range tx.Outputs {
			if out.CanBeUnlocked(address) && accumulated < amount {
				accumulated += out.Value
				unspentOuts[TxID] = append(unspentOuts[TxID], outIdx)

				if accumulated >= amount {
					break Work
				}
			}
		}
	}

	return accumulated, unspentOuts
}

func Handle(err error) {
	if err != nil {
		log.Panic(err)
	}
}
