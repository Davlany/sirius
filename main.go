package main

import (
	blockchain "bit/BlockChain"
	"flag"
	"fmt"
	"os"
	"runtime"
	"strconv"
)

type CommandLine struct {
	blockchain *blockchain.BlockChain
}

func (cli *CommandLine) PrintUsage() {
	fmt.Println("Usage:")
	fmt.Println("add -block BLOCK_DATA - add a block to the chain")
	fmt.Println("print - prints the blocks in the chain")
}

func (cli *CommandLine) ValidateArgs() {
	if len(os.Args) < 2 {
		cli.PrintUsage()
		runtime.Goexit()
	}
}

func (cli *CommandLine) PrintChain() {
	iter := cli.blockchain.Iterator()

	for {
		block := iter.Next()

		fmt.Printf("Previus Hash: %x\n", block.PrevHash)
		fmt.Printf("Hash: %x\n", block.Hash)
		fmt.Printf("Data: %s\n", block.Data)

		pow := blockchain.NewProof(block)
		fmt.Printf("PoW:%s\n", strconv.FormatBool(pow.Validate()))
		fmt.Println()

		if len(block.PrevHash) == 0 {
			break
		}
	}
}

func (cli *CommandLine) AddBlock(data string) {
	cli.blockchain.AddBlock(data)
	fmt.Println("Added Block!")
}

func (cli *CommandLine) Run() {
	cli.ValidateArgs()

	addBlockCMD := flag.NewFlagSet("add", flag.ExitOnError)
	printChainCMD := flag.NewFlagSet("print", flag.ExitOnError)
	addBlockData := addBlockCMD.String("block", "", "Block data")

	switch os.Args[1] {
	case "add":
		err := addBlockCMD.Parse(os.Args[2:])
		blockchain.Handle(err)
	case "print":
		err := printChainCMD.Parse(os.Args[2:])
		blockchain.Handle(err)
	default:
		cli.PrintUsage()
		runtime.Goexit()
	}

	if addBlockCMD.Parsed() {
		if *addBlockData == "" {
			addBlockCMD.Usage()
			runtime.Goexit()
		}
		cli.AddBlock(*addBlockData)
	}

	if printChainCMD.Parsed() {
		cli.PrintChain()
	}

}

func main() {
	defer os.Exit(0)
	chain := blockchain.InitBlockChain()
	defer chain.Database.Close()

	cli := CommandLine{chain}
	cli.Run()
}
