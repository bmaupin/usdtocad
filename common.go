package main

import "time"

const (
	fromCurrency = "USD"
	toCurrency   = "CAD"
)

func stripTimeFromDate(t time.Time) time.Time {
	return t.UTC().Truncate(time.Duration(time.Hour * 24))
}
