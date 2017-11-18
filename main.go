package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"sync"
	"time"

	"crypto/tls"
)

const (
	BTCMBaseURL   = "https://api.btcmarkets.net"
	BTCMMarketURL = "market"
	BTCMTickURL   = "tick"
	LTCCOUNT      = 100.0
	THRESHOLD     = 1.0
)

const (
	LOGDEBUG = iota
	LOGINFO
	LOGERROR
)

type Pair struct {
	Currency   string // Right value
	Instrument string // Left value
	Name       string
	Percent    float64
	market     Market
	cv         CurrVal
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

var (
	markets  map[string]Market
	loglevel int
	allArbs  map[string]CurrArb
)

type Arb struct {
	Diff float64 // the arbitrage value
}

type CurrArb struct {
	Arb *Arb
	Mut *sync.RWMutex
}

func main() {
	loglevel = LOGERROR

	// Initialise the global arb store.
	allArbs = make(map[string]CurrArb)

	// Initialise available markets.
	markets = make(map[string]Market)
	markets["BTCMarkets"] = Market{
		Name:    "BTCMarkets",
		BaseURL: BTCMBaseURL,
		TypeURL: BTCMMarketURL,
		TickURL: BTCMTickURL,
		CrypFee: 0.9978, // Percent
		DollFee: 0.9915,
	}
	var pairs []Pair

	btcMarkets := markets["BTCMarkets"]
	constructPairs(&pairs, "AUD", "BTC", btcMarkets)
	constructPairs(&pairs, "AUD", "LTC", btcMarkets)
	constructPairs(&pairs, "AUD", "BCH", btcMarkets)
	constructPairs(&pairs, "AUD", "XRP", btcMarkets)
	constructPairs(&pairs, "AUD", "ETH", btcMarkets)

	constructPairs(&pairs, "BTC", "ETH", btcMarkets)
	constructPairs(&pairs, "BTC", "LTC", btcMarkets)
	constructPairs(&pairs, "BTC", "BCH", btcMarkets)
	constructPairs(&pairs, "BTC", "XRP", btcMarkets)

	// Create a record for all of the arbitrage pairs in the global object>
	// These will be updated periodically.
	allArbs["LTC/BTC/AUD"] = CurrArb{Arb: &Arb{Diff: 0.00}, Mut: new(sync.RWMutex)}
	allArbs["BCH/BTC/AUD"] = CurrArb{Arb: &Arb{Diff: 0.00}, Mut: new(sync.RWMutex)}
	allArbs["XRP/BTC/AUD"] = CurrArb{Arb: &Arb{Diff: 0.00}, Mut: new(sync.RWMutex)}
	allArbs["ETH/BTC/AUD"] = CurrArb{Arb: &Arb{Diff: 0.00}, Mut: new(sync.RWMutex)}

	// Start calculating!
	ticker := time.NewTicker(time.Second * 20)
	go func() {
		// Catch any segfaults.
		defer func() {
			if r := recover(); r != nil {
				start(pairs)
			}
		}()
		for _ = range ticker.C {
			start(pairs)
		}
	}()

	// Wait forever.
	var w sync.WaitGroup
	w.Add(1)
	w.Wait()
}

func constructPairs(pairs *[]Pair, currency, instrument string, market Market) {
	*pairs = append(*pairs, Pair{
		Currency:   currency,
		Instrument: instrument,
		Name:       fmt.Sprintf("%s/%s", instrument, currency),
		market:     market,
	})
}

func start(pairs []Pair) {
	for i, _ := range pairs {
		val, err := getPair(pairs[i])
		if err != nil {
			fmt.Println(err.Error())
		}
		pairs[i].cv = val
		time.Sleep(1 * time.Second)
		printPair(pairs[i])
	}
	calcArbitrage(pairs)
}

func printPair(p Pair) {
	if loglevel == LOGDEBUG {
		fmt.Printf("PAIR: %s\n", p.Name)
		fmt.Printf("%+v\n\n", p.cv)
	}
}

func calcArbitrage(pairs []Pair) {
	// We need the best bid price for the current markets.
	curr := make(map[string]float64)
	for _, p := range pairs {
		curr[p.Name] = p.cv.BestBid
	}

	dollFee := pairs[0].market.DollFee
	crypFee := pairs[0].market.CrypFee
	name := pairs[0].market.Name

	fmt.Printf("=========== %s ===========\n", name)

	// LTC/AUD
	ltcVal := curr["LTC/AUD"] * dollFee
	ltcBtcVal := curr["LTC/BTC"] * crypFee * curr["BTC/AUD"] * dollFee
	ltcBtcDiff := getDiff(ltcVal, ltcBtcVal)
	updateArb("LTC/BTC/AUD", ltcBtcDiff)
	printVals(ltcVal, ltcBtcVal, "LTC/AUD", "LTC/BTC/AUD")

	// BCH/AUD
	bchVal := curr["BCH/AUD"] * dollFee
	bchBtcVal := curr["BCH/BTC"] * crypFee * curr["BTC/AUD"] * dollFee
	bchBtcDiff := getDiff(bchVal, bchBtcVal)
	updateArb("BCH/BTC/AUD", bchBtcDiff)
	printVals(bchVal, bchBtcVal, "BCH/AUD", "BCH/BTC/AUD")

	// XRP/AUD
	xrpVal := curr["XRP/AUD"] * dollFee
	xrpBtcVal := curr["XRP/BTC"] * crypFee * curr["BTC/AUD"] * dollFee
	xrpBtcDiff := getDiff(xrpVal, xrpBtcVal)
	updateArb("XRP/BTC/AUD", xrpBtcDiff)
	printVals(xrpVal, xrpBtcVal, "XRP/AUD", "XRP/BTC/AUD")

	// ETH/AUD
	ethVal := curr["ETH/AUD"] * dollFee
	ethBtcVal := curr["ETH/BTC"] * crypFee * curr["BTC/AUD"] * dollFee
	ethBtcDiff := getDiff(ethVal, ethBtcVal)
	updateArb("ETH/BTC/AUD", ethBtcDiff)
	printVals(ethVal, ethBtcVal, "ETH/AUD", "ETH/BTC/AUD")
}

func updateArb(name string, diff float64) {
	allArbs[name].Mut.Lock()
	allArbs[name].Arb.Diff = diff
	allArbs[name].Mut.Unlock()
}

func readArb(name string) float64 {
	allArbs[name].Mut.RLock()
	d := allArbs[name].Arb.Diff
	allArbs[name].Mut.RUnlock()
	return d
}

func printVals(pair, triple float64, pairName, tripName string) {
	fmt.Printf("===== %s =====\n", tripName)
	fmt.Printf("Value of %s: %.5f\n", pairName, pair)
	fmt.Printf("Value of %s: %.5f\n", tripName, triple)
	fmt.Printf("DIFF AFTER CONVERTING %.5f%%\n\n", readArb(tripName))
}

func getDiff(pair, trip float64) float64 {
	return (trip / pair * 100) - 100 // Convert to percent difference.
}

func formatTickUrl(market, t, ins, curr, tick string) string {
	return fmt.Sprintf("%s/%s/%s/%s/%s", market, t, ins, curr, tick)
}

func getPair(p Pair) (CurrVal, error) {
	resp := CurrVal{}

	url := formatTickUrl(p.market.BaseURL,
		p.market.TypeURL,
		p.Instrument,
		p.Currency,
		p.market.TickURL)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	rawResp, err := HttpClient().Do(req)
	defer rawResp.Body.Close()
	if err != nil {
		return resp, err
	}

	if friendErr := checkErr(rawResp); friendErr != nil {
		fmt.Println(friendErr.Error())
		return resp, err
	}

	body, err := ioutil.ReadAll(rawResp.Body)
	if err != nil {
		return resp, err
	}

	if err := json.Unmarshal(body, &resp); err != nil {
		return resp, err
	}

	return resp, err
}

// Client is an interface with the idea of wrapping an http.Client with extra
// functionality.
type Client interface {
	Do(r *http.Request) (*http.Response, error)
}

// Decorator wraps a Client with extra behaviour.
// Inspired by Tomas Senart (https://www.youtube.com/watch?v=xyDkyFjzFVc)
type Decorator func(Client) Client

// ClientFunc is the implementation of the Client interface.
type ClientFunc func(*http.Request) (*http.Response, error)

// Do performs the http request.
func (f ClientFunc) Do(r *http.Request) (*http.Response, error) {
	return f(r)
}

// Retry is a will retry an http request up to 'attempts' number of times,
// gradually increasing the retry wait time the more failed attempts.
func Retry(attempts int, backoff time.Duration) Decorator {
	return func(c Client) Client {
		return ClientFunc(func(r *http.Request) (res *http.Response, err error) {
			for i := 0; i <= attempts; i++ {
				if res, err = c.Do(r); err == nil {
					break
				}
				// We'll try again in a bit.
				time.Sleep(backoff * time.Duration(i))
			}
			return res, err
		})
	}
}

// Decorate takes a Client and wraps it with the provided decorators.
func Decorate(c Client, d ...Decorator) Client {
	dec := c
	for _, decFunc := range d {
		dec = decFunc(dec)
	}
	return dec
}

// IgnoreTlsErr is a that will prevent http client certificate errors when
// making an http request with a self-signed cert.
func IgnoreTlsErr() Decorator {
	return func(c Client) Client {
		// Ignore client certificate errors.
		if httpClient, ok := c.(*http.Client); ok {
			httpClient.Transport = &http.Transport{
				TLSClientConfig: &tls.Config{
					InsecureSkipVerify: true,
				},
			}
		}
		return ClientFunc(func(r *http.Request) (*http.Response, error) {
			return c.Do(r)
		})
	}
}

// HttpClient returns a decorated http client.
func HttpClient() Client {
	return Decorate(http.DefaultClient,
		Retry(1, time.Second),
	)
}

// checkErr returns a friendly error message for the given status code.
func checkErr(resp *http.Response) error {
	switch resp.StatusCode {
	case http.StatusBadRequest:
		return errors.New("Invalid options provided")
	case http.StatusUnauthorized:
		return errors.New("Not authorized to do that")
	case http.StatusForbidden:
		return errors.New("Forbidden from accessing that resource")
	case http.StatusNotFound:
		return errors.New("Missing that resource")
	case http.StatusInternalServerError:
		return errors.New("Internal error performing action")
	}
	return nil
}
