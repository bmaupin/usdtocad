package main

import (
	"bytes"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"time"
)

const (
	oxrUrlTemplate = "https://openexchangerates.org/api/historical/%s.json?app_id=%s"
	visaFxUrl      = "https://sandbox.api.visa.com/forexrates/v1/foreignexchangerates"
)

func getRateFromOXR(date time.Time) (float64, error) {
	url := fmt.Sprintf(oxrUrlTemplate,
		date.Format("2006-01-02"),
		os.Getenv("OXR_APP_ID"))

	resp, err := http.Get(url)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return 0, err
	}

	type OxrResponse struct {
		Base  string
		Rates map[string]float64
	}

	oxrResponse := &OxrResponse{}
	err = json.Unmarshal(body, &oxrResponse)
	if err != nil {
		return 0, err
	}

	return oxrResponse.Rates["CAD"], nil
}

func getRateFromVisa() (float64, error) {
	// Encode the user ID/pass using base64
	encodedUserIdPass := base64.StdEncoding.EncodeToString([]byte(os.Getenv("VISA_USER_PASS")))

	var jsonStr = []byte(`{
		"destinationCurrencyCode": "124",
		"sourceCurrencyCode": "840",
		"sourceAmount": "1"
		}`)
	req, err := http.NewRequest("POST", visaFxUrl, bytes.NewBuffer(jsonStr))
	// Get the response as JSON (default is XML)
	req.Header.Set("Accept", "application/json")
	// Required
	req.Header.Set("Authorization", "Basic "+encodedUserIdPass)
	// Required
	req.Header.Set("Content-Type", "application/json")

	// Load client cert
	cert, err := tls.LoadX509KeyPair("visa-client-cert.pem", "visa-client-key.pem")
	if err != nil {
		return 0, err
	}

	// Set up HTTPS client
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{Certificates: []tls.Certificate{cert}},
	}
	client := &http.Client{Transport: tr}

	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("Bad HTTP Response: %v", resp.Status)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return 0, err
	}

	type ExchangeRate struct {
		ConversionRate    string
		DestinationAmount string
	}

	exchangeRate := &ExchangeRate{}
	err = json.Unmarshal(body, &exchangeRate)
	if err != nil {
		return 0, err
	}

	visaRate, err := strconv.ParseFloat(exchangeRate.ConversionRate, 64)

	// TODO: remove this in production
	u, err := url.Parse(visaFxUrl)
	if err != nil {
		return 0, err
	}
	if u.Host == "sandbox.api.visa.com" && visaRate <= 1 {
		return 0, fmt.Errorf("Error: fake rate returned from visa API sandbox")
	}

	return visaRate, nil
}
