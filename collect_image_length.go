package main

import (
	"net/http"
	"strconv"
	"time"
)

func collectImageLength() {
	type Job struct {
		UrlId  int64  `db:"url_id"`
		Url    string `db:"url"`
		Length int
	}
	jobs := make(chan Job)

	go func() {
		for {
			rows, err := db.Queryx(`SELECT
				url_id, url
				FROM urls
				WHERE length IS NULL
				LIMIT 4096
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

	results := make(chan Job, 4096)
	go func() {
		count := 0
		tx := db.MustBegin()
		commit := func() {
			ce(tx.Commit(), "commit")
			tx = db.MustBegin()
		}
		commitTicker := time.NewTicker(time.Second * 30)
		for {
			select {
			case <-commitTicker.C:
				commit()
			case job := <-results:
				_, err := tx.Exec(`UPDATE urls
				SET length = $1
				WHERE url_id = $2
				`,
					job.Length,
					job.UrlId,
				)
				ce(err, "update urls")
				count++
				if count%100 == 0 {
					pt("%d\n", count)
					pt("%d %d\n", job.UrlId, job.Length)
					commit()
				}
			}
		}
	}()

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
			if length == "" {
				return
			}
			l, err := strconv.Atoi(length)
			ce(err, "parse length")
			job.Length = l
			results <- job
		}()
	}

}
