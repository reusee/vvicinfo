package main

import (
	"crypto/sha512"
	"database/sql"
	"io"
	"net/http"
	"strconv"
	"sync"
	"sync/atomic"
	"time"
)

func init() {
	http.DefaultClient.Timeout = time.Second * 8
}

func collectImageInfos() {
	var n int64
	closeTicker := make(chan bool)
	defer func() {
		close(closeTicker)
	}()
	ticker := time.NewTicker(time.Second * 5)
	go func() {
		for {
			select {
			case <-ticker.C:
				pt("%d\n", atomic.LoadInt64(&n))
			case <-closeTicker:
				return
			}
		}
	}()

	failCount := make(map[string]int)
	failCountLock := new(sync.Mutex)

collect:

	// select urls
	t0 := time.Now()
	var urls []string
	err := db.Select(&urls, `SELECT url
			FROM images 
			JOIN image_vars USING(image_id)
			WHERE hashed = false
			LIMIT 2048
			`,
	)
	ce(err, "select")

	// filter and dedup
	urlSet := make(map[string]struct{})
	for _, url := range urls {
		if failCount[url] > 3 {
			continue
		}
		urlSet[url] = struct{}{}
	}
	if len(urlSet) == 0 {
		return
	}
	pt("%d urls\n", len(urlSet))

	// collect
	wg := new(sync.WaitGroup)
	wg.Add(len(urlSet))
	sem := make(chan bool, semSize)
	for url := range urlSet {
		url := url
		sem <- true
		go func() {
			defer func() {
				<-sem
				wg.Done()
				atomic.AddInt64(&n, 1)
			}()
			err := collectImageInfo(url)
			if err != nil {
				//panic(err)
				pt("%v\n", err)
				failCountLock.Lock()
				failCount[url]++
				failCountLock.Unlock()
			}
		}()
	}
	wg.Wait()

	pt("collect %d in %v\n", len(urlSet), time.Now().Sub(t0))
	goto collect

}

func collectImageInfo(url string) (err error) {
	defer ct(&err)

	// 先查数据库，看有没有已经hash的
	var hash []byte
	var length int
	err = db.QueryRow(`SELECT 
		sha512_16k, length
		FROM images
		WHERE url = $1
		AND sha512_16k IS NOT NULL
		AND length IS NOT NULL
		LIMIT 1
		`,
		url,
	).Scan(&hash, &length)
	if err == nil { // 有
		ce(updateImageInfo(url, hash, length), "update image info")
		return
	} else if err != sql.ErrNoRows { // 出错
		ce(err, "check existing hash")
	}

	retry := 10
get:
	resp, err := http.Get(url)
	if err != nil {
		if retry > 0 {
			retry--
			goto get
		}
		ce(err, "get image")
	}
	defer resp.Body.Close()

	contentLen := resp.Header.Get("Content-Length")
	if contentLen == "" {
		contentLen = "0"
	}
	length, err = strconv.Atoi(contentLen)
	ce(err, "parse content len: %s", contentLen)

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
	ce(updateImageInfo(url, sum, length), "update image info")
	return
}

func updateImageInfo(url string, hash []byte, length int) (err error) {
	defer ct(&err)
	var imageIds []int64
	err = db.Select(&imageIds, `UPDATE images
		SET sha512_16k = $1, length = $3
		WHERE url = $2
		AND (sha512_16k IS NULL OR length IS NULL)
		RETURNING image_id
		`,
		hash,
		url,
		length,
	)
	ce(err, "update hash")
	for _, imageId := range imageIds {
		_, err = db.Exec(`UPDATE image_vars
			SET hashed = true
			WHERE image_id = $1
			`,
			imageId,
		)
		ce(err, "update images")
	}
	return
}
