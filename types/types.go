package types

import "strconv"

type Config struct {
	Chain                 string      `json:"chain"`
	Denom                 string      `json:"denom"`
	Prefix                string      `json:"prefix"`
	GasPerByte            int64       `json:"gas_per_byte"`
	BaseGas               int64       `json:"base_gas"`
	TimeoutHeight         int64       `json:"timeout_height"`
	EpochLength           int64       `json:"epoch_length"`
	NumTopics             int         `json:"num_topics"`
	WorkersPerTopic       int         `json:"workers_per_topic"`
	ReputersPerTopic      int         `json:"reputers_per_topic"`
	CreateTopicsSameBlock bool        `json:"create_topics_same_block"`
	TimeoutMinutes        int64       `json:"timeout_minutes"`
	Gas                   GasConfig   `json:"gas"`
	Nodes                 NodesConfig `json:"nodes"`
}

type GasConfig struct {
	Low       int64 `json:"low"`
	Precision int64 `json:"precision"`
}

type NodesConfig struct {
	RPC  []string `json:"rpc"`
	API  string   `json:"api"`
	GRPC string   `json:"grpc"`
}

type AccountInfo struct {
	Sequence      string `json:"sequence"`
	AccountNumber string `json:"account_number"`
}

type AccountResult struct {
	Account AccountInfo `json:"account"`
}

type BalanceResult struct {
	Balances   []Coin     `json:"balances"`
	Pagination Pagination `json:"pagination"`
}

type NextTopicIdResult struct {
	TopicId string `json:"next_topic_id"`
}

type Coin struct {
	Denom  string `json:"denom"`
	Amount string `json:"amount"`
}

type Pagination struct {
	NextKey string `json:"next_key"`
	Total   string `json:"total"`
}

type Actor struct {
	Name   string
	Addr   string
	Params *TransactionParams
}

func (a Actor) String() string {
	return a.Name
}

// generates an actors name from seed and index
func GetActorName(actorIndex int) string {
	return "run_actor" + strconv.Itoa(actorIndex)
}

type UnfulfilledReputerNoncesResult struct {
	Nonces ReputerRequestNonces `json:"nonces"`
}

type ReputerRequestNonces struct {
	Nonces []ReputerRequestNonce `json:"nonces"`
}

type UnfulfilledWorkerNoncesResult struct {
	Nonces Nonces `json:"nonces"`
}

type Nonces struct {
	Nonces []Nonce `json:"nonces"`
}

type Nonce struct {
	BlockHeight string `json:"block_height"`
}

type ReputerRequestNonce struct {
	ReputerNonce ReputerNonce `json:"reputer_nonce"`
}

type ReputerNonce struct {
	BlockHeight string `json:"block_height"`
}

type InferencesAtBlockResult struct {
	Inferences Inferences `json:"inferences"`
}

type Inferences struct {
	Inferences []*Inference `json:"inferences"`
}

type Inference struct {
	TopicId     string `json:"topic_id"`
	BlockHeight string `json:"block_height"`
	Inferer     string `json:"inferer"`
	Value       string `json:"value"`
	ExtraData   []byte `json:"extra_data"`
	Proof       string `json:"proof"`
}
