package blockchain

type Transaction struct {
	ID      []byte
	Inputs  []TxInput
	Outputs []TxOutput
}

type TxInput struct {
	ID  []byte
	Out int
	Sig string
}

type TxOutput struct {
	Value  int
	PubKey string
}
