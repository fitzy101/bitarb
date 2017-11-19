package main

import (
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/mux"

	"github.com/fitzy101/bitarb/arb"
	t "github.com/fitzy101/bitarb/types"
	"github.com/fitzy101/bitarb/ui"
)

const (
	THRESHOLD = 1.0
)

const (
	LOGDEBUG = iota
	LOGINFO
	LOGERROR
)

var (
	markets  map[string]t.Market
	loglevel int
)

func main() {
	loglevel = LOGERROR

	// Initialise the global arb store.
	arb.AllArbs = make(map[string]t.CurrArb)

	// Initialise available markets.
	markets = make(map[string]t.Market)
	markets["BTCMarkets"] = t.Market{
		Name:    "BTCMarkets",
		BaseURL: arb.BTCMBaseURL,
		TypeURL: arb.BTCMMarketURL,
		TickURL: arb.BTCMTickURL,
		CrypFee: 0.9978, // Percent
		DollFee: 0.9915,
	}
	var pairs []t.Pair

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

	// TODO: support other exchanges.

	// Create a record for all of the arbitrage pairs in the global object>
	// These will be updated periodically.
	arb.AllArbs["LTC/BTC/AUD"] = t.CurrArb{Arb: &t.Arb{Diff: 0.00}, Mut: new(sync.RWMutex), PairName: "LTC/AUD"}
	arb.AllArbs["BCH/BTC/AUD"] = t.CurrArb{Arb: &t.Arb{Diff: 0.00}, Mut: new(sync.RWMutex), PairName: "BCH/AUD"}
	arb.AllArbs["XRP/BTC/AUD"] = t.CurrArb{Arb: &t.Arb{Diff: 0.00}, Mut: new(sync.RWMutex), PairName: "XRP/AUD"}
	arb.AllArbs["ETH/BTC/AUD"] = t.CurrArb{Arb: &t.Arb{Diff: 0.00}, Mut: new(sync.RWMutex), PairName: "ETH/AUD"}

	// Start calculating!
	ticker := time.NewTicker(time.Second * 10)
	go func() {
		// Catch any segfaults.
		defer func() {
			if r := recover(); r != nil {
				// Do nothing for now, keep going.
			}
		}()
		for _ = range ticker.C {
			arb.Start(pairs)
		}
	}()

	// We can boot the api now after setting it up, and accept requests.
	httpServer()
}

func constructPairs(pairs *[]t.Pair, currency, instrument string, market t.Market) {
	*pairs = append(*pairs, t.Pair{
		Currency:   currency,
		Instrument: instrument,
		Name:       fmt.Sprintf("%s/%s", instrument, currency),
		Market:     market,
	})
}

func httpServer() {
	server := &http.Server{
		Addr:           ":5000",
		Handler:        newRouter(),
		ReadTimeout:    30 * time.Second,
		WriteTimeout:   30 * time.Second,
		MaxHeaderBytes: 1 << 20, // Equivalent to 1 mb.
	}
	fmt.Printf("Bitarb serving on %v.\n", server.Addr)
	if err := server.ListenAndServe(); err != nil {
		panic(err)
	}
}

func newRouter() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		if r.Method == "OPTIONS" {
			return
		}

		router := mux.NewRouter()
		router.NotFoundHandler = notFound()
		router.Handle("/", ui.RootHandler())
		router.ServeHTTP(w, r)
	})
}

// notFound is returned when the requested url was not matched
// on any of the other routers. This is the generic 404 response.
func notFound() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprintf(w, "{\"error\":\"Page not found - %v %v\"}", r.Method, r.URL.Path)
		return
	})
}
