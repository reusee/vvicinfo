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

type UrlInfo struct {
	Url        string
	UrlId      int64 `db:"url_id"`
	Sha512_16k []byte
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
		var infos []*UrlInfo
		err := db.Select(&infos, `SELECT url, h.url_id
			FROM not_hashed h
			LEFT JOIN urls u ON h.url_id = u.url_id
			LIMIT 2048
			`,
		)
		ce(err, "select")
		var filtered []*UrlInfo
		for _, info := range infos {
			if failCount[info.UrlId] > 3 {
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
					failCount[info.UrlId]++
					failCountLock.Unlock()
				}
			}()
		}
		wg.Wait()
		tx.Commit()
		pt("collect %d in %v\n", len(infos), time.Now().Sub(t0))
	}

}

func hashImage(info *UrlInfo, tx *sqlx.Tx) (err error) {
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
	_, err = tx.Exec(`UPDATE urls 
		SET sha512_16k = $1
		WHERE url_id = $2`,
		sum,
		info.UrlId)
	ce(err, "update hash")
	_, err = tx.Exec(`DELETE FROM not_hashed
		WHERE url_id = $1
		`,
		info.UrlId,
	)
	ce(err, "delete from not_hashed")
	return
}
