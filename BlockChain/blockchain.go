package blockchain

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"log"

	"github.com/dgraph-io/badger"
	"github.com/sirupsen/logrus"
)

const DB_PATH = "./tmp/blocks"

type Block struct {
	Hash     []byte
	Data     []byte
	PrevHash []byte
	Nonce    int
}

type BlockChain struct {
	LastHash []byte
	Database *badger.DB
}

type BlockChainIterator struct {
	CurrentHash []byte
	Database    *badger.DB
}

func CreateBlock(data string, previusHash []byte) *Block {
	block := &Block{[]byte{}, []byte(data), previusHash, 0}
	pow := NewProof(block)
	nonce, hash := pow.Run()
	block.Hash = hash
	block.Nonce = nonce
	return block
}

func (chain *BlockChain) AddBlock(data string) {
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

	newBlock := CreateBlock(data, lastHash)

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

func Deserialize(data []byte) *Block {
	var block Block

	decoder := gob.NewDecoder(bytes.NewReader(data))
	err := decoder.Decode(&block)

	Handle(err)

	return &block
}

func Genesis() *Block {
	return CreateBlock("Sirius", []byte{})
}

func InitBlockChain() *BlockChain {
	var lastHash []byte
	logrus.SetLevel(logrus.ErrorLevel)
	opts := badger.DefaultOptions(DB_PATH)
	opts.Dir = DB_PATH
	opts.Logger = logrus.StandardLogger()
	opts.ValueDir = DB_PATH

	db, err := badger.Open(opts)
	Handle(err)

	err = db.Update(func(txn *badger.Txn) error {
		if _, err := txn.Get([]byte("lh")); err == badger.ErrKeyNotFound {
			fmt.Println("No existing blockchain found")
			genesis := Genesis()
			fmt.Println("Genesis proved")
			err := txn.Set(genesis.Hash, genesis.Serialize())
			Handle(err)
			err = txn.Set([]byte("lh"), genesis.Hash)
			lastHash = genesis.Hash

			return err
		} else {
			item, err := txn.Get([]byte("lh"))
			Handle(err)
			err = item.Value(func(val []byte) error {
				lastHash = val
				return nil
			})
			return err
		}
	})
	Handle(err)

	blockchain := BlockChain{lastHash, db}
	return &blockchain
}

func Handle(err error) {
	if err != nil {
		log.Panic(err)
	}
}
