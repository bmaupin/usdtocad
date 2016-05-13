package main

import (
	"errors"
	"fmt"
	"os"
	"time"

	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
)

const (
	dbCollRates       = "rates"
	dbCollRatePairs   = "ratepairs"
	dbCollRateSources = "ratesources"
	dbSourceOXR       = "openexchangerates.org"
	dbSourceVisa      = "visa.com"
	timeFormat        = "2006-01-02"
)

// ErrRateExists is returned by AddRateValue when the rate to add already exists
var ErrRateExists = errors.New("rate already exists")

type (
	Rate struct {
		ID           bson.ObjectId `bson:"_id,omitempty"`
		RatePairID   bson.ObjectId `bson:"rate_pair_id"`
		RateSourceID bson.ObjectId `bson:"rate_source_id"`
		Date         time.Time
		Value        float64
	}

	RateSource struct {
		ID   bson.ObjectId `bson:"_id,omitempty"`
		Name string
	}

	RatePair struct {
		ID     bson.ObjectId `bson:"_id,omitempty"`
		From   string
		To     string
		Amount float64
	}
)

type DataStore struct {
	session *mgo.Session
}

func newDataStore(url string) (*DataStore, error) {
	var err error
	ds := &DataStore{}

	if url == "" {
		url = os.Getenv("OPENSHIFT_MONGODB_DB_URL")
	}

	ds.session, err = mgo.Dial(url)
	if err != nil {
		return ds, err
	}

	// Optional. Switch the session to a monotonic behavior.
	ds.session.SetMode(mgo.Monotonic, true)

	return ds, nil
}

// Returns a collection from the datastore given its name
func (ds *DataStore) GetCollection(name string) *mgo.Collection {
	c := ds.session.DB(os.Getenv("OPENSHIFT_APP_NAME")).C(name)

	return c
}

// Returns a rate pair given its from currency, to currency, and amount
func (ds *DataStore) GetOrAddRatePair(from string, to string, amount float64) (RatePair, error) {
	ratePairsColl := ds.GetCollection(dbCollRatePairs)
	ratePair := RatePair{}

	err := ratePairsColl.Find(bson.M{"from": from, "to": to, "amount": amount}).One(&ratePair)
	if err == mgo.ErrNotFound {
		err = ratePairsColl.Insert(&RatePair{
			From:   from,
			To:     to,
			Amount: amount,
		})
		if err != nil {
			return ratePair, err
		}

		ratePair = RatePair{}
		err = ratePairsColl.Find(bson.M{"from": from, "to": to, "amount": amount}).One(&ratePair)
		if err != nil {
			return ratePair, err
		}

	} else if err != nil {
		return ratePair, err
	}

	return ratePair, nil
}

// Returns a rate source given its name
func (ds *DataStore) GetOrAddRateSource(name string) (RateSource, error) {
	rateSourcesColl := ds.GetCollection(dbCollRateSources)
	rateSource := RateSource{}
	err := rateSourcesColl.Find(bson.M{"name": name}).One(&rateSource)
	if err == mgo.ErrNotFound {
		err = rateSourcesColl.Insert(&RateSource{
			Name: name,
		})
		if err != nil {
			return rateSource, err
		}

		rateSource = RateSource{}
		err = rateSourcesColl.Find(bson.M{"name": name}).One(&rateSource)
		if err != nil {
			return rateSource, err
		}

	} else if err != nil {
		return rateSource, err
	}

	return rateSource, nil
}

// Returns the value of a rate given a rate pair, rate source, and date
func (ds *DataStore) GetOrAddRateValue(ratePair RatePair, rateSource RateSource, date time.Time) (float64, error) {
	rate := Rate{}
	ratesColl := ds.GetCollection(dbCollRates)

	err := ratesColl.Find(bson.M{
		"rate_pair_id":   ratePair.ID,
		"rate_source_id": rateSource.ID,
		"date":           date}).One(&rate)
	if err == mgo.ErrNotFound {
		rateValue := 0.0
		today, err := time.Parse(timeFormat, time.Now().Format(timeFormat))
		if err != nil {
			return 0, err
		}

		if rateSource.Name == dbSourceOXR {
			rateValue, err = getRateFromOXR(date)
			if err != nil {
				return 0, err
			}

		} else if rateSource.Name == dbSourceVisa {
			rateValue, err = getRateFromVisa(date)
			if err != nil {
				return 0, err
			}
		}

		// Don't save today's rate in the DB since it will change throughout the day
		if date == today {
			return rateValue, nil
		}

		err = ds.AddRateValue(ratePair, rateSource, date, rateValue)
		if err != nil {
			return 0, err
		}

		rate = Rate{}
		err = ratesColl.Find(bson.M{
			"rate_pair_id":   ratePair.ID,
			"rate_source_id": rateSource.ID,
			"date":           date}).One(&rate)
		if err != nil {
			return 0, err
		}

	} else if err != nil {
		return 0, err
	}

	return rate.Value, nil
}

// For a particular rate pair and ratesource, get the most recent rate and
// return its date
func (ds *DataStore) GetMostRecentRateDate(ratePair RatePair, rateSource RateSource) (time.Time, error) {
	rate := Rate{}
	ratesColl := ds.GetCollection(dbCollRates)

	err := ratesColl.Find(bson.M{
		"rate_pair_id":   ratePair.ID,
		"rate_source_id": rateSource.ID}).Sort("-date").One(&rate)

	if err == mgo.ErrNotFound {
		return time.Now(), fmt.Errorf("Error: no rates found for source %v", rateSource.Name)
	}

	return rate.Date, nil
}

func (ds *DataStore) AddRateValue(ratePair RatePair, rateSource RateSource, date time.Time, value float64) error {
	rate := Rate{}
	ratesColl := ds.GetCollection(dbCollRates)

	err := ratesColl.Find(bson.M{
		"rate_pair_id":   ratePair.ID,
		"rate_source_id": rateSource.ID,
		"date":           date}).One(&rate)

	if err == nil {
		return ErrRateExists

	} else if err != mgo.ErrNotFound {
		return err
	}

	err = ratesColl.Insert(&Rate{
		RatePairID:   ratePair.ID,
		RateSourceID: rateSource.ID,
		Date:         date,
		Value:        value,
	})
	if err != nil {
		return err
	}

	return nil
}
