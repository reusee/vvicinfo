package main

import (
	"crypto/sha512"
	//"github.com/jmoiron/sqlx"
	"io"
	"net/http"
	"sync"
	"sync/atomic"
	"time"
)

type ImageInfo struct {
	Url        string `db:"url"`
	Sha512_16k []byte `db:"sha512_16k"`
}

func hashImages() {
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

	for {
		t0 := time.Now()
		var infos []*ImageInfo
		err := db.Select(&infos, `SELECT url
			FROM images 
			JOIN image_vars USING(image_id)
			WHERE hashed = false
			LIMIT 2048
			`,
		)
		ce(err, "select")
		var filtered []*ImageInfo
		for _, info := range infos {
			if failCount[info.Url] > 3 {
				continue
			}
			filtered = append(filtered, info)
		}
		infos = filtered
		if len(infos) == 0 {
			break
		}
		pt("select %d infos\n", len(infos))
		wg := new(sync.WaitGroup)
		wg.Add(len(infos))
		sem := make(chan bool, semSize)
		for _, info := range infos {
			info := info
			sem <- true
			go func() {
				defer func() {
					<-sem
					wg.Done()
					atomic.AddInt64(&n, 1)
				}()
				err := hashImage(info)
				if err != nil {
					panic(err)
					pt("%v\n", err)
					failCountLock.Lock()
					failCount[info.Url]++
					failCountLock.Unlock()
				}
			}()
		}
		wg.Wait()
		pt("collect %d in %v\n", len(infos), time.Now().Sub(t0))
	}

}

func hashImage(info *ImageInfo) (err error) {
	defer ct(&err)

	tx := db.MustBegin()
	defer func() {
		ce(tx.Commit(), "commit")
	}()

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

	contentLen := resp.Header.Get("Content-Length")
	if contentLen == "" {
		contentLen = "0"
	}

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
	var imageIds []int64
	err = tx.Select(&imageIds, `UPDATE images
		SET sha512_16k = $1, length = $3
		WHERE url = $2
		RETURNING image_id
		`,
		sum,
		info.Url,
		contentLen,
	)
	ce(err, "update hash")
	for _, imageId := range imageIds {
		_, err = tx.Exec(`UPDATE image_vars
			SET hashed = true
			WHERE image_id = $1
			`,
			imageId,
		)
		ce(err, "update images")
	}
	return
}
