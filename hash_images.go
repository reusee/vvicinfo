package main

import (
	"crypto/sha512"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"
)

type UrlInfo struct {
	Url        string
	UrlId      int64 `db:"url_id"`
	Sha512_16k []byte
}

func hashImages() {
	rowsChan := make(chan *UrlInfo, 8)
	go func() {
		cnt := 0
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
				if cnt%128 == 0 {
					ce(tx.Commit(), "commit")
					tx = db.MustBegin()
				}
			case <-tick.C:
				ce(tx.Commit(), "commit")
				tx = db.MustBegin()
			}
		}
	}()

	n := 0

start:
	var rows []*UrlInfo
	err := db.Select(&rows, `SELECT url, url_id FROM urls
		WHERE sha512_16k IS NULL
		ORDER BY url_id DESC
		LIMIT 4096
		`)
	ce(err, "select urls")

	wg := new(sync.WaitGroup)
	wg.Add(len(rows))
	sem := make(chan bool, semSize)
	for _, row := range rows {
		pt("%7d %s\n", n, row.Url)
		n++
		sem <- true
		go func() {
			defer func() {
				<-sem
				wg.Done()
			}()
			_ = hashImage(row, rowsChan)
		}()
	}
	wg.Wait()

	if len(rows) > 0 {
		goto start
	}

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
	if err != nil {
		if err != nil {
			if retry > 0 {
				retry--
				goto get
			}
			ce(err, "read body")
		}
	}

	sum := h.Sum(nil)
	info.Sha512_16k = sum[:]
	rowsChan <- info
	return
}

func downloadImage(image Image, nBytes *int64, fileExists map[string]bool, fileExistsLock *sync.Mutex) (err error) {
	defer ct(&err)
	// get image
	body, err := getBody(image.Url)
	ce(err, "get image content %s %d", image.Url, image.GoodId)
	// sum
	sumAry := sha512.Sum512(body)
	sum := sumAry[:]
	sumHex := fmt.Sprintf("%x", sum)
	// write to tmp
	var exists bool
	fileExistsLock.Lock()
	if _, exists = fileExists[sumHex]; exists { // only one goroutine should be write to file
	} else {
		fileExists[sumHex] = true
	}
	fileExistsLock.Unlock()
	if !exists {
		fileName := filepath.Join("images", fmt.Sprintf("%x%s", sum,
			filepath.Ext(image.Url)))
		tmpFile, err := os.Create(fileName + ".tmp")
		ce(err, "create tmp file")
		_, err = tmpFile.Write(body)
		ce(err, "write tmp file")
		tmpFile.Close()
		// rename
		ce(os.Rename(fileName+".tmp", fileName), "rename file")
	}
	// update db
	_, err = db.Exec(`UPDATE images SET sha512 = $1
				WHERE good_id = $2`,
		sum, image.GoodId)
	ce(err, "update hash sum")
	// stat
	atomic.AddInt64(nBytes, int64(len(body)))
	return
}
