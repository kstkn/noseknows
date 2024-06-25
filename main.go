package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
)

const munich = "DEMUNC"

const birch = "Betula"
const haselnut = "Corylus"
const grasses = "Poaceae"

var germany *time.Location

type timestamp struct {
	time.Time
}

func (p *timestamp) UnmarshalJSON(bytes []byte) error {
	var raw int64
	err := json.Unmarshal(bytes, &raw)

	if err != nil {
		fmt.Printf("error decoding timestamp: %s\n", err)
		return err
	}

	p.Time = time.Unix(raw, 0).In(germany)
	return nil
}

type data struct {
	From  timestamp `json:"from"`
	To    timestamp `json:"to"`
	Value float64   `json:"value"`
}

type measurement struct {
	Name     string `json:"polle"`
	Location string `json:"location"`
	Data     []data `json:"data"`
}
type response struct {
	From         timestamp     `json:"from"`
	To           timestamp     `json:"to"`
	Measurements []measurement `json:"measurements"`
}

func createUrl(from *time.Time, to *time.Time, locations []string, allergens []string) url.URL {
	uri := url.URL{
		Scheme: "https",
		Host:   "epin.lgl.bayern.de",
		Path:   "/api/measurements",
	}
	query := uri.Query()
	if from != nil {
		query.Set("from", fmt.Sprintf("%d", from.Unix()))
	}
	if to != nil {
		query.Set("to", fmt.Sprintf("%d", to.Unix()))
	}
	query.Set("locations", strings.Join(locations, ","))
	query.Set("pollen", strings.Join(allergens, ","))
	uri.RawQuery = query.Encode()
	return uri
}

func main() {
	var err error
	germany, err = time.LoadLocation("Europe/Berlin")
	if err != nil {
		panic(err)
	}

	http.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
		from := time.Now().Add(-24 * time.Hour).In(germany)
		uri := createUrl(&from, nil, []string{munich}, []string{birch, haselnut, grasses})

		resp, err := http.Get(uri.String())
		if err != nil {
			log.Error(err)
			w.WriteHeader(500)
			w.Write([]byte(fmt.Sprintf("# ERROR %v", err)))
			return
		}
		defer resp.Body.Close()

		var respBody response
		if err = json.NewDecoder(resp.Body).Decode(&respBody); err != nil {
			log.Error(err)
		}

		now := time.Now().In(germany)
		lastMidnight := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, germany)

		w.Write([]byte("# HELP pollen_history_static pollen historical measurements\n# TYPE pollen_history_static gauge\n"))
		w.Write([]byte("# HELP pollen_history pollen historical measurements\n# TYPE pollen_history gauge\n"))

		latestDate := from
		latestValues := make(map[string]map[string]float64)
		for _, m := range respBody.Measurements {
			for _, d := range m.Data {
				if d.To.Equal(latestDate) || d.To.After(latestDate) {
					latestDate = d.To.Time
					if _, ok := latestValues[m.Location]; !ok {
						latestValues[m.Location] = make(map[string]float64)
					}
					latestValues[m.Location][m.Name] = d.Value
				}
				t := "today_"
				if d.To.Before(lastMidnight) {
					t = "yesterday_"
				}
				w.Write([]byte(fmt.Sprintf(`pollen_history_static{location="%s",name="%s",time="%s"} %.2f`, m.Location, m.Name, t+d.To.Time.Format("15"), d.Value)))
				w.Write([]byte("\n"))
			}
		}
		for l, m := range latestValues {
			for n, v := range m {
				w.Write([]byte(fmt.Sprintf(`pollen{location="%s",name="%s"} %.2f`, l, n, v)))
				w.Write([]byte("\n"))
			}
		}

	})
	err = http.ListenAndServe(":9092", nil)
	if err != nil {
		log.Fatal(err)
	}
}
