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

func collectDetailPages() {
	var ids []int64
	err := db.Select(&ids, `SELECT good_id FROM images a
		LEFT JOIN goods b
		GROUP BY good_id
		HAVING COUNT(*) = 1
		WHERE status = 1
		ORDER BY good_id DESC
		`)
	ce(err, "select ids")

	wg := new(sync.WaitGroup)
	wg.Add(len(ids))
	sem := make(chan bool, 8)
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
			Imgs string // 图片
			Desc string // 描述html
		}
	}
	err = json.NewDecoder(resp.Body).Decode(&data)
	ce(err, "decode")
	tx := db.MustBegin()
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
	ce(tx.Commit(), "commit")
	return
}

func saveGoodImage(tx *sqlx.Tx, good_id int64, url string) (err error) {
	if url != "" {
		_, err = tx.Exec(`INSERT INTO images (
					good_id,
					url
				) VALUES (
					?,
					?
				) ON DUPLICATE KEY UPDATE good_id=good_id`, good_id, url)
	}
	return
}
