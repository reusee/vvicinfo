package main

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
)

func collectRankings() {
	resp, err := http.Get("http://www1.vvic.com/data/ranking.jsonp?callback=ranking&_=1471443802930")
	ce(err, "get")
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	ce(err, "read body")

	body = body[8 : len(body)-1]
	var data struct {
		List1 []struct {
			List []struct {
				Id string `json:"id"`
			} `json:"list"`
		} `json:"list1"`
		List2 []struct {
			List []struct {
				Id string `json:"id"`
			} `json:"list"`
		} `json:"list2"`
	}
	ce(json.Unmarshal(body, &data), "unmarshal")

	rank := 1
	tx := db.MustBegin()
	defer func() {
		ce(tx.Commit(), "commit")
	}()

	_, err = tx.Exec(`UPDATE shops SET rank = NULL`)
	ce(err, "clear ranks")

	for _, list := range data.List1 {
		for _, entry := range list.List {
			_, err = tx.Exec(`UPDATE shops SET rank = $1
			WHERE shop_id = $2
			`,
				rank,
				entry.Id,
			)
			ce(err, "update rank")
			rank++
		}
	}
	for _, list := range data.List2 {
		for _, entry := range list.List {
			_, err = tx.Exec(`UPDATE shops SET rank = $1
			WHERE shop_id = $2
			`,
				rank,
				entry.Id,
			)
			ce(err, "update rank")
			rank++
		}
	}

}
