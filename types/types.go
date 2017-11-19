package types

import (
	"sync"
)

type Pair struct {
	Currency   string
	Instrument string
	Name       string
	Percent    float64
	Market     Market
	Cv         CurrVal
}

type CurrVal struct {
	BestBid    float64 `json:"bestBid"`
	BestAsk    float64 `json:"bestAsk"`
	LastPrice  float64 `json:"lastPrice"`
	Currency   string  `json:"currency"`
	Instrument string  `json:"instrument"`
	Timestamp  int64   `json:"timestamp"`
	Vol24Hr    float64 `json:"volume24h"`
	crFee      float64
	doFee      float64
}

type Market struct {
	Name    string
	BaseURL string
	TypeURL string
	TickURL string
	CrypFee float64
	DollFee float64
}

type Arb struct {
	Diff    float64 // the arbitrage value
	PairVal float64
	TripVal float64
}

type CurrArb struct {
	PairName string
	Arb      *Arb
	Mut      *sync.RWMutex
}
