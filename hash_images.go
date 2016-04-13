package main

import (
	"crypto/sha512"
	"io"
	"net/http"
	"sync"
	"time"
)

type UrlInfo struct {
	Url        string
	UrlId      int64 `db:"url_id"`
	Sha512_16k []byte
}

func hashImages() {

	// updater
	rowsChan := make(chan *UrlInfo, 1024)
	go func() {
		cnt := 0
		n := 0
		tx := db.MustBegin()
		tick := time.NewTicker(time.Second * 10)
		for {
			select {
			case row := <-rowsChan:
				tx.MustExec(`UPDATE urls SET sha512_16k = $1
				WHERE url_id = $2`,
					row.Sha512_16k,
					row.UrlId)
				cnt++
				n++
				if cnt%128 == 0 {
					ce(tx.Commit(), "commit")
					tx = db.MustBegin()
					cnt = 0
				}
			case <-tick.C:
				ce(tx.Commit(), "commit")
				tx = db.MustBegin()
				cnt = 0
				pt("%d\n", n)
			}
		}
	}()

	// job provider
	jobs := make(chan *UrlInfo, 30000)
	go func() {
		for {
			rows, err := db.Queryx(`SELECT url, url_id FROM urls
				WHERE url_id IN ( SELECT distinct url_id FROM ( SELECT u.url_id FROM urls u
					LEFT JOIN images i
					ON u.url_id = i.url_id
					LEFT JOIN goods g
					ON g.good_id = i.good_id
					WHERE g.status = 1
					AND g.added_at > '2016-01-01'
					AND u.sha512_16k IS NULL
					LIMIT 1024
				) as tmp)
			`)
			ce(err, "query")
			pt("query done\n")
			for rows.Next() {
				row := new(UrlInfo)
				ce(rows.StructScan(&row), "scan")
				jobs <- row
			}
			ce(rows.Err(), "rows")

		}
	}()

	// worker
	wg := new(sync.WaitGroup)
	sem := make(chan bool, semSize)
	for row := range jobs {
		wg.Add(1)
		sem <- true
		go func() {
			defer func() {
				<-sem
				wg.Done()
			}()
			ce(hashImage(row, rowsChan), "hash")
		}()
	}
	wg.Wait()

	time.Sleep(time.Second * 30)
}

func hashImage(info *UrlInfo, rowsChan chan *UrlInfo) (err error) {
	defer ct(&err)
	retry := 10
get:
	resp, err := http.Get(info.Url)
	if err != nil {
		if retry > 0 {
			retry--
			goto get
		}
		ce(err, "get image")
	}
	defer resp.Body.Close()

	h := sha512.New()
	_, err = io.CopyN(h, resp.Body, 16384)
	if err == io.EOF {
		err = nil
	}
	if err != nil {
		if retry > 0 {
			retry--
			goto get
		}
		ce(err, "read body")
	}

	sum := h.Sum(nil)
	info.Sha512_16k = sum[:]
	rowsChan <- info
	return
}
