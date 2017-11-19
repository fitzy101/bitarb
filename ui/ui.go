package ui

import (
	"bytes"
	"fmt"
	"html/template"

	"github.com/fitzy101/bitarb/arb"
)

type triple struct {
	Diff     string
	PairName string
	PairVal  float64
	TripName string
	TripVal  float64
}

var (
	RootTemplate *template.Template
	has_loaded   = false // will be updated after PrepTemplates is called.
)

// RenderRoot returns the root-level html page as an io.Reader served with the
// file server.
func RenderRoot() ([]byte, error) {
	var data []byte
	if !has_loaded {
		if err := prepTemplates(); err != nil {
			return data, err
		}
	}
	data, err := renderRoot()
	return data, err
}

// prepTemplates reads the template files from disk into memory on load.
func prepTemplates() error {
	// Parse the template in preparation for execution.
	r, err := template.ParseFiles("ui/pages/root.html")
	if err != nil {
		return err
	}
	has_loaded = true
	RootTemplate = r
	return nil
}

func renderRoot() ([]byte, error) {
	trips := getCurrVals()
	data := struct {
		Title  string
		Market string
		Ltc    triple
		Bch    triple
		Xrp    triple
		Eth    triple
	}{
		Title:  "BITARB.COM",
		Ltc:    trips["LTC/AUD"],
		Bch:    trips["BCH/AUD"],
		Xrp:    trips["XRP/AUD"],
		Eth:    trips["ETH/AUD"],
		Market: "btcmarkets.net",
	}
	buff := new(bytes.Buffer)
	err := RootTemplate.Execute(buff, data)
	return buff.Bytes(), err
}

func getCurrVals() map[string]triple {
	ret := make(map[string]triple)
	// We want this to always be in the same order.
	for k, v := range arb.AllArbs {
		v.Mut.RLock()
		ret[v.PairName] = triple{
			Diff:     fmt.Sprintf("%.5f", v.Arb.Diff),
			PairName: v.PairName,
			PairVal:  v.Arb.PairVal,
			TripName: k,
			TripVal:  v.Arb.TripVal,
		}
		v.Mut.RUnlock()
	}

	return ret
}
