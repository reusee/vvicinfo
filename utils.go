package main

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
)

func decodeFromUrl(path string, target interface{}) (err error) {
	retry := 20
retry:
	pageResp, err := http.Get(path)
	if err != nil {
		if retry < 0 {
			return err
		}
		retry--
		goto retry
	}
	defer pageResp.Body.Close()
	err = json.NewDecoder(pageResp.Body).Decode(target)
	if err != nil {
		if retry < 0 {
			return err
		}
		retry--
		goto retry
	}
	return nil
}

func getBody(url string) (body []byte, err error) {
	retry := 5
get:
	resp, err := http.Get(url)
	if err != nil {
		if retry > 0 {
			retry--
			goto get
		}
		return nil, err
	}
	defer resp.Body.Close()
	body, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		if retry > 0 {
			retry--
			goto get
		}
		return nil, err
	}
	return body, nil
}
