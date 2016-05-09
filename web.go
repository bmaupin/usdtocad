package main

import (
	"fmt"
	"net/http"
	"os"
	"time"
)

func main() {
	http.HandleFunc("/", hello)
	bind := fmt.Sprintf("%s:%s", os.Getenv("OPENSHIFT_GO_IP"), os.Getenv("OPENSHIFT_GO_PORT"))
	fmt.Printf("listening on %s...", bind)
	err := http.ListenAndServe(bind, nil)
	if err != nil {
		panic(err)
	}
}

func hello(res http.ResponseWriter, req *http.Request) {
	err := showRates(res)
	if err != nil {
		fmt.Fprintln(res, err)
	}
}

func showRates(res http.ResponseWriter) error {
	ds, err := newDataStore()
	defer ds.session.Close()
	if err != nil {
		return err
	}

	fromCurrency := "USD"
	toCurrency := "CAD"
	ratesOXR := []float64{}
	ratesVisa := []float64{}

	fmt.Fprintf(res, "%v to %v\n\n", fromCurrency, toCurrency)
	fmt.Fprintf(res, "%-16v%-16v%-16vVisa difference\n", "", "OXR", "Visa")

	ratePairUSDCAD, err := ds.GetRatePair(fromCurrency, toCurrency, 1)
	if err != nil {
		return err
	}
	rateSourceVisa, err := ds.GetRateSource(dbSourceVisa)
	if err != nil {
		return err
	}
	rateSourceOXR, err := ds.GetRateSource(dbSourceOXR)
	if err != nil {
		return err
	}

	mostRecentVisaRateDate, err := ds.GetMostRecentRateDate(ratePairUSDCAD, rateSourceVisa)
	if err != nil {
		return err
	}

	for i := -31; i <= -1; i++ {
		date := stripTimeFromDate(mostRecentVisaRateDate.AddDate(0, 0, i))

		rateVisa, err := ds.GetRateValue(ratePairUSDCAD, rateSourceVisa, date)
		if err != nil {
			// TODO: stop if we hit an error with the Visa API
			break
		}
		ratesVisa = append(ratesVisa, rateVisa)

		rateOXR, err := ds.GetRateValue(ratePairUSDCAD, rateSourceOXR, date)
		if err != nil {
			return err
		}
		ratesOXR = append(ratesOXR, rateOXR)

		fmt.Fprintf(res, "%-16v%-16.7v%-16.7v%+.2g%%\n",
			date.Format(timeFormat),
			rateOXR,
			rateVisa,
			rateVisa/rateOXR*100-100,
		)
	}

	averateRateOXR := average(ratesOXR)
	averateRateVisa := average(ratesVisa)

	fmt.Fprintf(res, "\n%-16v%-16.7v%-16.7v%+.2g%%\n",
		"Average",
		averateRateOXR,
		averateRateVisa,
		averateRateVisa/averateRateOXR*100-100,
	)

	return nil
}

func stripTimeFromDate(t time.Time) time.Time {
	return t.UTC().Truncate(time.Duration(time.Hour * 24))
}

func average(f []float64) float64 {
	total := 0.0
	for i := 0; i < len(f); i++ {
		total += f[i]
	}

	return total / float64(len(f))
}
