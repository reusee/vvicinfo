package main

import (
	"crypto/sha512"
	"github.com/jmoiron/sqlx"
	"io"
	"net/http"
	"sync"
	"sync/atomic"
	"time"
)

type ImageInfo struct {
	ImageId    int64  `db:"image_id"`
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

	failCount := make(map[int64]int)
	failCountLock := new(sync.Mutex)

	for {
		t0 := time.Now()
		var infos []*ImageInfo
		err := db.Select(&infos, `SELECT image_id, url
			FROM not_hashed h
			LEFT JOIN images i USING(image_id)
			LIMIT 2048
			`,
		)
		ce(err, "select")
		var filtered []*ImageInfo
		for _, info := range infos {
			if failCount[info.ImageId] > 3 {
				continue
			}
			filtered = append(filtered, info)
		}
		infos = filtered
		if len(infos) == 0 {
			break
		}
		pt("select %d infos\n", len(infos))
		tx := db.MustBegin()
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
				err := hashImage(info, tx)
				if err != nil {
					pt("%v\n", err)
					failCountLock.Lock()
					failCount[info.ImageId]++
					failCountLock.Unlock()
				}
			}()
		}
		wg.Wait()
		tx.Commit()
		pt("collect %d in %v\n", len(infos), time.Now().Sub(t0))
	}

}

func hashImage(info *ImageInfo, tx *sqlx.Tx) (err error) {
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
	_, err = tx.Exec(`UPDATE images
		SET sha512_16k = $1, length = $3
		WHERE image_id = $2`,
		sum,
		info.ImageId,
		contentLen,
	)
	ce(err, "update hash")
	_, err = tx.Exec(`DELETE FROM not_hashed
		WHERE image_id = $1
		`,
		info.ImageId,
	)
	ce(err, "delete from not_hashed")
	return
}
