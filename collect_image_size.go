package main

import (
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"net/http"
	"sync"
	"sync/atomic"
	"time"
)

func collectImageSize() {
	var total int64

	http.DefaultClient.Timeout = time.Minute

collect:
	var infos []struct {
		UrlId int    `db:"url_id"`
		Url   string `db:"url"`
	}
	ce(db.Select(&infos, `SELECT
		url_id, url
		FROM urls
		WHERE width IS NULL
		LIMIT 512
		`,
	), "select info")
	pt("select %d infos\n", len(infos))

	sem := make(chan struct{}, 8)
	wg := new(sync.WaitGroup)
	wg.Add(len(infos))
	tx := db.MustBegin()
	for _, info := range infos {
		info := info
		sem <- struct{}{}
		go func() {
			defer func() {
				wg.Done()
				<-sem
			}()
			pt("%d %s\n", atomic.AddInt64(&total, 1), info.Url)
			resp, err := http.Get(info.Url)
			if err != nil {
				return
			}
			defer resp.Body.Close()

			img, _, err := image.Decode(resp.Body)
			if err != nil {
				return
			}
			bounds := img.Bounds()
			_, err = tx.Exec(`UPDATE urls
				SET width = $1, height = $2
				WHERE url_id = $3
				`,
				bounds.Dx(),
				bounds.Dy(),
				info.UrlId,
			)
			ce(err, "update urls")
		}()
	}
	wg.Wait()

	ce(tx.Commit(), "commit")
	pt("collect %d images\n", len(infos))

	goto collect
}
