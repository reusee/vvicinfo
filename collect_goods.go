package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"

	"github.com/PuerkitoBio/goquery"
	"github.com/jmoiron/sqlx"
)

func collectGoods() {
	var ids []int64
	err := db.Select(&ids, `SELECT g.good_id FROM goods g
		LEFT JOIN images i
		ON g.good_id = i.good_id
		WHERE i.url_id IS NULL
		ORDER BY g.good_id DESC
		`)
	ce(err, "select ids")
	pt("%d ids\n", len(ids))

	wg := new(sync.WaitGroup)
	wg.Add(len(ids))
	sem := make(chan bool, semSize)
	l := len(ids)
	for i, id := range ids {
		id := id
		i := i
		sem <- true
		go func() {
			defer func() {
				<-sem
				wg.Done()
			}()
			n, err := collectDetailPage(id)
			ce(err, "collect %d", id)
			pt("%10d / %10d collected, %10d images, id %10d\n",
				i, l, n, id)
		}()
	}
	wg.Wait()
}

func collectDetailPage(id int64) (n int, err error) {
	defer ct(&err)
	pagePath := fmt.Sprintf("http://www.vvic.com/api/item/%d", id)
	retry := 10
get:
	resp, err := http.Get(pagePath)
	if err != nil {
		if retry > 0 {
			retry--
			goto get
		}
		ce(err, "get page %s", pagePath)
	}
	defer resp.Body.Close()
	var data struct {
		Code int
		Data struct {
			Imgs   string // 图片
			Desc   string // 描述html
			Status int    // 上下架状态
		}
	}
	err = json.NewDecoder(resp.Body).Decode(&data)
	if err != nil {
		if retry > 0 {
			retry--
			goto get
		}
		ce(err, "decode json %s", pagePath)
	}
	ce(withTx(db, func(tx *sqlx.Tx) (err error) {
		defer ct(&err)
		for _, imgPath := range strings.Split(data.Data.Imgs, ",") {
			if imgPath == "" {
				continue
			}
			if !strings.HasPrefix(imgPath, "http:") {
				imgPath = "http:" + imgPath
			}
			ce(saveGoodImage(tx, id, imgPath), "save image url")
			n++
		}
		doc, err := goquery.NewDocumentFromReader(strings.NewReader(data.Data.Desc))
		ce(err, "goquery doc")
		doc.Find("img").Each(func(i int, se *goquery.Selection) {
			imgSrc, _ := se.Attr("src")
			if !strings.HasPrefix(imgSrc, "http") {
				return
			}
			ce(saveGoodImage(tx, id, imgSrc), "save image url")
			n++
		})
		_, err = tx.Exec(`UPDATE goods 
			SET status = $1 
			WHERE good_id = $2`,
			data.Data.Status,
			id)
		ce(err, "update status")
		return
	}), "tx")
	return
}

func saveGoodImage(tx *sqlx.Tx, good_id int64, url string) (err error) {
	defer ct(&err)
	if url != "" {
		_, err := tx.Exec(`INSERT INTO urls (url) VALUES ($1)
			ON CONFLICT (url) DO NOTHING`,
			url)
		ce(err, "insert url")
		_, err = tx.Exec(`INSERT INTO images (
					good_id,
					url_id
				) VALUES (
					$1,
					(SELECT url_id FROM urls WHERE url = $2)
				)
				ON CONFLICT (good_id, url_id) DO NOTHING`,
			good_id,
			url)
		ce(err, "insert image")
	}
	return
}
