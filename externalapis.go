package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"time"

	"golang.org/x/net/html"

	"github.com/bmaupin/go-util/htmlutil"
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

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("Bad HTTP Response: %v", resp.Status)
	}

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

func getRateFromVisa(t time.Time) (float64, error) {
	v := url.Values{}
	v.Add("fromCurr", "CAD")
	v.Add("toCurr", "USD")
	v.Add("fee", "0")
	v.Add("exchangedate", t.Format("01/02/2006"))

	url := fmt.Sprintf("https://usa.visa.com/support/consumer/travel-support/exchange-rate-calculator.html/?%s", v.Encode())

	client := &http.Client{}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		log.Fatalln(err)
	}

	req.Header.Set("User-Agent", "Golang")

	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}

	doc, err := html.Parse(resp.Body)
	if err != nil {
		return 0, err
	}

	/*
		This is what we're looking for:
		<p class="exchgName">
			<span class="results"><strong>1</strong> United States Dollar = <strong>1.258974</strong> Canadian Dollar</span>
		</p><br>
	*/
	resultsNode, err := htmlutil.GetFirstHtmlNode(doc, "span", "class", "results")
	if err != nil {
		return 0, err
	}

	strongNodes, err := htmlutil.GetAllHtmlNodes(resultsNode, "strong", "", "")
	if err != nil {
		return 0, err
	}

	for _, strongNode := range strongNodes {
		if strongNode.FirstChild.Data == "1" {
			continue
		}
		visaRate, err := strconv.ParseFloat(strongNode.FirstChild.Data, 64)
		if err != nil {
			return 0, err
		}
		return visaRate, nil
	}

	return 0, fmt.Errorf("Exchange rate not found")
}
