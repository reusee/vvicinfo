package main

import (
	"net/http"
	"sync/atomic"
)

func collectImageLength() {
	type Job struct {
		UrlId int64  `db:"url_id"`
		Url   string `db:"url"`
	}
	jobs := make(chan Job)

	go func() {
		for {
			rows, err := db.Queryx(`SELECT
				url_id, url
				FROM urls
				WHERE length IS NULL
				LIMIT 1024
				`,
			)
			ce(err, "query")
			for rows.Next() {
				var job Job
				ce(rows.StructScan(&job), "scan")
				jobs <- job
			}
			ce(rows.Err(), "rows err")
		}
	}()

	var count int64
	sem := make(chan struct{}, 64)
	for job := range jobs {
		job := job
		sem <- struct{}{}
		go func() {
			defer func() {
				<-sem
			}()
			resp, err := http.Head(job.Url)
			if err != nil {
				return
			}
			length := resp.Header.Get("Content-Length")
			_, err = db.Exec(`UPDATE urls
			SET length = $1
			WHERE url_id = $2
			`,
				length,
				job.UrlId,
			)
			ce(err, "update urls")
			n := atomic.AddInt64(&count, 1)
			if n%100 == 0 {
				pt("%d\n", n)
			}
		}()
	}
}
