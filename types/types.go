package types

import (
	cryptorand "crypto/rand"
	"math/big"
	"math/rand/v2"
	"strconv"

	"cosmossdk.io/math"
)

type Config struct {
	ChainID               string              `json:"chain_id"`
	Denom                 string              `json:"denom"`
	Prefix                string              `json:"prefix"`
	GasPerByte            uint64              `json:"gas_per_byte"`
	BaseGas               uint64              `json:"base_gas"`
	GasAdjustment         float64             `json:"gas_adjustment"`
	OverrideFee           uint64              `json:"override_fee"`
	MaxFees               uint64              `json:"max_fees"`
	EpochLength           int64               `json:"epoch_length"`
	NumTopics             int                 `json:"num_topics"`
	InferersPerTopic      int                 `json:"inferers_per_topic"`
	ForecastersPerTopic   int                 `json:"forecasters_per_topic"`
	ReputersPerTopic      int                 `json:"reputers_per_topic"`
	CreateTopicsSameBlock bool                `json:"create_topics_same_block"`
	TimeoutMinutes        int64               `json:"timeout_minutes"`
	Nodes                 NodesConfig         `json:"nodes"`
	Research              ResearchConfig      `json:"research"`
	BasicActivity         BasicActivityConfig `json:"basic_activity"`
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
	Name           string
	Addr           string
	TxParams       *TransactionParams
	ResearchParams *ResearchParams
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

type WorkerAttributedValue struct {
	Worker string `json:"worker"`
	Value  string `json:"value"`
}

type WithheldWorkerAttributedValue struct {
	Worker string `json:"worker"`
	Value  string `json:"value"`
}

type OneOutInfererForecasterValues struct {
	Forecaster          string                          `json:"forecaster"`
	OneOutInfererValues []WithheldWorkerAttributedValue `json:"one_out_inferer_values"`
}

type ValueBundle struct {
	TopicId                       string                          `json:"topic_id"`
	ReputerRequestNonce           *ReputerRequestNonce            `json:"reputer_request_nonce,omitempty"`
	Reputer                       string                          `json:"reputer"`
	ExtraData                     []byte                          `json:"extra_data"`
	CombinedValue                 string                          `json:"combined_value"`
	InfererValues                 []WorkerAttributedValue         `json:"inferer_values"`
	ForecasterValues              []WorkerAttributedValue         `json:"forecaster_values"`
	NaiveValue                    string                          `json:"naive_value"`
	OneOutInfererValues           []WithheldWorkerAttributedValue `json:"one_out_inferer_values"`
	OneOutForecasterValues        []WithheldWorkerAttributedValue `json:"one_out_forecaster_values"`
	OneInForecasterValues         []WorkerAttributedValue         `json:"one_in_forecaster_values"`
	OneOutInfererForecasterValues []OneOutInfererForecasterValues `json:"one_out_inferer_forecaster_values"`
}

// API-friendly version of ValueBundle
type GetNetworkInferencesAtBlockResponse struct {
	NetworkInferences *ValueBundle `json:"network_inferences"`
}

// RESEARCH MODULE

type ResearchConfig struct {
	InitialPrice           float64      `json:"initial_price"`
	Drift                  float64      `json:"drift"`
	Volatility             float64      `json:"volatility"`
	BaseExperienceFactor   float64      `json:"base_experience_factor"`
	ExperienceGrowth       float64      `json:"experience_growth"`
	OutperformValue        float64      `json:"outperform_value"`
	ConsistentOutperformer bool         `json:"consistent_outperformer"`
	Topic                  TopicConfig  `json:"topic"`
	GlobalParams           GlobalParams `json:"global_params"`
}

type GlobalParams struct {
	MaxSamplesToScaleScores uint64 `json:"max_samples_to_scale_scores"`
}

type TopicConfig struct {
	LossMethod               string `json:"loss_method"`
	EpochLength              int64  `json:"epoch_length"`
	GroundTruthLag           int64  `json:"ground_truth_lag"`
	WorkerSubmissionWindow   int64  `json:"worker_submission_window"`
	PNorm                    string `json:"p_norm"`
	AlphaRegret              string `json:"alpha_regret"`
	AllowNegative            bool   `json:"allow_negative"`
	Epsilon                  string `json:"epsilon"`
	MeritSortitionAlpha      string `json:"merit_sortition_alpha"`
	ActiveInfererQuantile    string `json:"active_inferer_quantile"`
	ActiveForecasterQuantile string `json:"active_forecaster_quantile"`
	ActiveReputerQuantile    string `json:"active_reputer_quantile"`
}

type ResearchParams struct {
	Volatility         float64
	Error              float64
	Bias               float64
	BiasWithVolatility float64 // used by inferers
	ContextSensitivity float64
	Outperform         bool
	LossFunction       string
}

type GroundTruthState struct {
	CumulativeReturn float64
	CurrentPrice     float64
	LastReturn       float64
}

// BASIC ACTIVITY MODULE

type BasicActivityConfig struct {
	NumActors      int             `json:"num_actors"`
	RandWalletSeed int64           `json:"rand_wallet_seed"`
	TxsPerBlock    Range[uint32]   `json:"txs_per_block"`
	SendAmount     Range[math.Int] `json:"send_amount"`
	RefundAmount   math.Int        `json:"refund_amount"`
}

type Range[T intType] struct {
	Min T `json:"min"`
	Max T `json:"max"`
}

type intType interface {
	uint32 | math.Int
}

func (rng Range[T]) RandInBetween() T {
	var t T
	tAny := any(t)
	switch tAny.(type) {
	case math.Int:
		lo := any(rng.Min).(math.Int).BigInt()
		hi := any(rng.Max).(math.Int).BigInt()

		n := new(big.Int).Sub(hi, lo)
		n = n.Add(n, big.NewInt(1))
		r, err := cryptorand.Int(cryptorand.Reader, n)
		if err != nil {
			panic(err)
		}
		r = r.Add(r, lo)

		return any(math.NewIntFromBigInt(r)).(T)
	case uint32:
		lo := any(rng.Min).(uint32)
		hi := any(rng.Max).(uint32)
		return any(rand.N(hi-lo+1) + lo).(T)
	default:
		panic("unsupported type for Range")
	}
}
