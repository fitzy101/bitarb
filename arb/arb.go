package arb

import (
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	t "github.com/fitzy101/bitarb/types"
)

const (
	BTCMBaseURL   = "https://api.btcmarkets.net"
	BTCMMarketURL = "market"
	BTCMTickURL   = "tick"
)

var (
	AllArbs map[string]t.CurrArb
)

// Start is the only public function, which handles the retrieval and calculation
// of the arbitrage for each pair.
func Start(pairs []t.Pair) {
	for i, _ := range pairs {
		val, err := getPair(pairs[i])
		if err != nil {
			fmt.Println(err.Error())
		}
		pairs[i].Cv = val
		time.Sleep(1 * time.Second) // Avoid rate limits.
	}
	calcArbitrage(pairs)
}

func calcArbitrage(pairs []t.Pair) {
	// We need the best bid price for the current markets.
	curr := make(map[string]float64)
	for _, p := range pairs {
		curr[p.Name] = p.Cv.BestBid
	}

	dollFee := pairs[0].Market.DollFee
	crypFee := pairs[0].Market.CrypFee
	name := pairs[0].Market.Name

	fmt.Printf("=========== %s ===========\n", name)

	// LTC/AUD
	ltcVal := curr["LTC/AUD"] * dollFee
	ltcBtcVal := curr["LTC/BTC"] * crypFee * curr["BTC/AUD"] * dollFee
	ltcBtcDiff := getDiff(ltcVal, ltcBtcVal)
	updateArb("LTC/BTC/AUD", ltcVal, ltcBtcVal, ltcBtcDiff)
	printVals(ltcVal, ltcBtcVal, "LTC/AUD", "LTC/BTC/AUD")

	// BCH/AUD
	bchVal := curr["BCH/AUD"] * dollFee
	bchBtcVal := curr["BCH/BTC"] * crypFee * curr["BTC/AUD"] * dollFee
	bchBtcDiff := getDiff(bchVal, bchBtcVal)
	updateArb("BCH/BTC/AUD", bchVal, bchBtcVal, bchBtcDiff)
	printVals(bchVal, bchBtcVal, "BCH/AUD", "BCH/BTC/AUD")

	// XRP/AUD
	xrpVal := curr["XRP/AUD"] * dollFee
	xrpBtcVal := curr["XRP/BTC"] * crypFee * curr["BTC/AUD"] * dollFee
	xrpBtcDiff := getDiff(xrpVal, xrpBtcVal)
	updateArb("XRP/BTC/AUD", xrpVal, xrpBtcVal, xrpBtcDiff)
	printVals(xrpVal, xrpBtcVal, "XRP/AUD", "XRP/BTC/AUD")

	// ETH/AUD
	ethVal := curr["ETH/AUD"] * dollFee
	ethBtcVal := curr["ETH/BTC"] * crypFee * curr["BTC/AUD"] * dollFee
	ethBtcDiff := getDiff(ethVal, ethBtcVal)
	updateArb("ETH/BTC/AUD", ethVal, ethBtcVal, ethBtcDiff)
	printVals(ethVal, ethBtcVal, "ETH/AUD", "ETH/BTC/AUD")
}

func updateArb(name string, pairval, tripval, diff float64) {
	AllArbs[name].Mut.Lock()
	AllArbs[name].Arb.PairVal = pairval
	AllArbs[name].Arb.TripVal = tripval
	AllArbs[name].Arb.Diff = diff
	AllArbs[name].Mut.Unlock()
}

func readArb(name string) float64 {
	AllArbs[name].Mut.RLock()
	d := AllArbs[name].Arb.Diff
	AllArbs[name].Mut.RUnlock()
	return d
}

func printVals(pair, triple float64, pairName, tripName string) {
	fmt.Printf("===== %s =====\n", tripName)
	fmt.Printf("Value of %s: %.5f\n", pairName, pair)
	fmt.Printf("Value of %s: %.5f\n", tripName, triple)
	fmt.Printf("Difference: %.5f%%\n\n", readArb(tripName))
}

func getDiff(pair, trip float64) float64 {
	return (trip / pair * 100) - 100 // Convert to percent difference.
}

func formatTickUrl(market, t, ins, curr, tick string) string {
	return fmt.Sprintf("%s/%s/%s/%s/%s", market, t, ins, curr, tick)
}

// TODO: currently this is not catering for different market bodies etc.
// It will need to be 'pluggable' with different API responses. Shouldn't be too
// much work as its quite low down.
// getPair makes the HTTP request to the market.
func getPair(p t.Pair) (t.CurrVal, error) {
	resp := t.CurrVal{}

	url := formatTickUrl(p.Market.BaseURL,
		p.Market.TypeURL,
		p.Instrument,
		p.Currency,
		p.Market.TickURL)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	rawResp, err := httpClient().Do(req)
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

/// Below is all functionatility that could be moved to another package.

// Client is an interface with the idea of wrapping an http.Client with extra
// functionality.
type Client interface {
	Do(r *http.Request) (*http.Response, error)
}

// decorator wraps a Client with extra behaviour.
// Inspired by Tomas Senart (https://www.youtube.com/watch?v=xyDkyFjzFVc)
type decorator func(Client) Client

// ClientFunc is the implementation of the Client interface.
type ClientFunc func(*http.Request) (*http.Response, error)

// Do performs the http request.
func (f ClientFunc) Do(r *http.Request) (*http.Response, error) {
	return f(r)
}

// retry is a will retry an http request up to 'attempts' number of times,
// gradually increasing the retry wait time the more failed attempts.
func retry(attempts int, backoff time.Duration) decorator {
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

// decorate takes a Client and wraps it with the provided decorators.
func decorate(c Client, d ...decorator) Client {
	dec := c
	for _, decFunc := range d {
		dec = decFunc(dec)
	}
	return dec
}

// ignoreTlsErr is a that will prevent http client certificate errors when
// making an http request with a self-signed cert.
func ignoreTlsErr() decorator {
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

// httpClient returns a decorated http client.
func httpClient() Client {
	return decorate(http.DefaultClient,
		retry(1, time.Second),
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
