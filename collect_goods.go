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
	err := db.Select(&ids, `SELECT good_id FROM goods
		WHERE good_id NOT IN (
			SELECT DISTINCT good_id FROM images)
		ORDER BY good_id DESC
		`)
	ce(err, "select ids")
	pt("%d ids\n", len(ids))

	wg := new(sync.WaitGroup)
	wg.Add(len(ids))
	sem := make(chan bool, 16)
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
	resp, err := http.Get(pagePath)
	ce(err, "get page")
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
	ce(err, "decode")
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
		_, err = tx.Exec(`UPDATE goods SET status = ? WHERE good_id = ?`,
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
		res, err := tx.Exec(`INSERT INTO urls (url) VALUES (?)
			ON DUPLICATE KEY UPDATE url_id = LAST_INSERT_ID(url_id)`,
			url)
		ce(err, "insert url")
		url_id, err := res.LastInsertId()
		ce(err, "get last insert id")
		_, err = tx.Exec(`INSERT INTO images (
					good_id,
					url_id
				) VALUES (
					?,
					?
				) ON DUPLICATE KEY UPDATE good_id=good_id`, good_id, url_id)
		ce(err, "insert image")
	}
	return
}
