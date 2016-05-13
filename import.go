package main

import (
	"flag"
	"fmt"
	"time"
)

var mongodbUser = flag.String("user", "admin", "MongoDB username")
var mongodbPass = flag.String("pass", "", "MongoDB password")

func main() {
	flag.Parse()

	err := importRates()
	if err != nil && err != ErrRateExists {
		fmt.Println(err)
	} else {
		fmt.Println("Import complete")
	}
}

func importRates() error {
	// Start from yesterday and move backward 30 days
	for i := -1; i >= -31; i-- {
		date := stripTimeFromDate(time.Now().AddDate(0, 0, i))

		rate, err := getRateFromVisa(date)
		if err != nil {
			return err
		}

		ds, err := newDataStore(fmt.Sprintf("%v:%v@127.0.0.1/usdtocad", *mongodbUser, *mongodbPass))
		if err != nil {
			return err
		}

		ratePairUSDCAD, err := ds.GetOrAddRatePair(fromCurrency, toCurrency, 1)
		if err != nil {
			return err
		}
		rateSourceVisa, err := ds.GetOrAddRateSource(dbSourceVisa)
		if err != nil {
			return err
		}

		err = ds.AddRateValue(ratePairUSDCAD, rateSourceVisa, date, rate)
		if err != nil {
			return err
		}
	}

	return nil
}
