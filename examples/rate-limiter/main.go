// SPDX-License-Identifier: CC0-1.0
// Creative Commons Zero v1.0 Universal - No Rights Reserved
// <https://creativecommons.org/publicdomain/zero/1.0>
//
// To the extent possible under law, the Teal.Finance/Garcon contributors
// have waived all copyright and related/neighboring rights to this
// file "high-level/main.go" to be freely used without any restriction.

// this example is an adapation of thise usage:
// https://github.com/teal-finance/rainbow/blob/main/pkg/provider/deribit/deribit.go
package main

import (
	"fmt"
	"time"

	"github.com/teal-finance/emo"
	"github.com/teal-finance/garcon"
)

var log = emo.NewZone("app")

// adaptiveMinSleepTime to rate limit the Deribit API.
// https://www.deribit.com/kb/deribit-rate-limits
// "Each sub-account has a rate limit of 100 in a burst or 20 requests per second".
const adaptiveMinSleepTime = 1 * time.Millisecond

// Hour at which the options expires = 8:00 UTC.
const Hour = 8

// maxBytesToRead prevents wasting memory/CPU when receiving an abnormally huge response from Deribit API.
const maxBytesToRead = 2_000_000

func main() {
	ar := garcon.NewAdaptiveRate("Deribit", adaptiveMinSleepTime)
	count := 0
	for i := 0; i < 1000; i++ {
		instruments, err := query(ar, "BTC")
		if err != nil {
			log.Fatalf(err.Error())
		}
		count += instruments
		instruments, err = query(ar, "ETH")
		if err != nil {
			log.Fatalf(err.Error())
		}
		count += instruments
		instruments, err = query(ar, "SOL")
		if err != nil {
			log.Fatalf(err.Error())
		}
		count += instruments
	}
	fmt.Printf("fetched %d instruments from Deribit \n", count)

	/*

		optionsBTC, err := p.fillOptions(instruments, 5)
		if err != nil {
			return nil, err
		}

		instruments, err = p.query("ETH")
		if err != nil {
			return nil, err
		}

		optionsETH, err := p.fillOptions(instruments, 5)
		if err != nil {
			return nil, err
		}

		instruments, err = p.query("SOL")
		if err != nil {
			return nil, err
		}*/

}

func query(ar garcon.AdaptiveRate, coin string) (int, error) {
	const api = "https://deribit.com/api/v2/public/get_instruments?currency="
	const opts = "&expired=false&kind=option"
	url := api + coin + opts
	log.Info("Deribit " + url)

	var result instrumentsResult
	err := ar.Get(coin, url, &result, maxBytesToRead)
	if err != nil {
		return 0, err
	}

	return len(result.Result), nil
}

type instrumentsResult struct {
	Result []instrument `json:"result"`
}

type instrument struct {
	OptionType           string  `json:"option_type"`
	InstrumentName       string  `json:"instrument_name"`
	Kind                 string  `json:"kind"`
	SettlementPeriod     string  `json:"settlement_period"`
	QuoteCurrency        string  `json:"quote_currency"`
	BaseCurrency         string  `json:"base_currency"`
	MinTradeAmount       float64 `json:"min_trade_amount"`
	MakerCommission      float64 `json:"maker_commission"`
	Strike               float64 `json:"strike"`
	TickSize             float64 `json:"tick_size"`
	TakerCommission      float64 `json:"taker_commission"`
	ExpirationTimestamp  int64   `json:"expiration_timestamp"`
	CreationTimestamp    int64   `json:"creation_timestamp"`
	ContractSize         float64 `json:"contract_size"`
	BlockTradeCommission float64 `json:"block_trade_commission"`
	IsActive             bool    `json:"is_active"`
}
