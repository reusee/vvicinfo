package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/PuerkitoBio/goquery"
	"github.com/jmoiron/sqlx"
)

var zeroImagesIds = make(map[int64]int)
var zeroImagesIdsLock = new(sync.Mutex)

func collectGoods() {
	var total uint64
do:
	pt("select good ids to fetch images\n")
	var ids, filtered []int64
	err := db.Select(&ids, `SELECT good_id FROM images_not_collected`)
	ce(err, "select ids")
	for _, id := range ids {
		if zeroImagesIds[id] > 5 {
			continue
		}
		filtered = append(filtered, id)
	}
	ids = filtered
	if len(ids) == 0 {
		return
	}
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
			if err != nil {
				pt("%v\n", err)
			} else {
				pt("%10d / %10d / %10d collected, %10d images, id %10d\n",
					i, l, atomic.AddUint64(&total, 1), n, id)
			}
		}()
	}
	wg.Wait()

	goto do
}

func collectDetailPage(id int64) (n int, err error) {
	defer ct(&err)
	pagePath := fmt.Sprintf("http://www.vvic.com/api/item/%d", id)
	retry := 50
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
			Size   string // 尺码
			Color  string // 颜色
			Is_tx  int
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
		nImages := 0
		for _, imgPath := range strings.Split(data.Data.Imgs, ",") {
			if imgPath == "" {
				continue
			}
			if !strings.HasPrefix(imgPath, "http:") {
				imgPath = "http:" + imgPath
			}
			ce(saveGoodImage(tx, id, imgPath), "save image url")
			n++
			nImages++
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
			nImages++
		})
		_, err = tx.Exec(`UPDATE goods 
			SET status = $1, sizes = $2, colors = $3, tuixian = $5
			WHERE good_id = $4`,
			data.Data.Status,
			data.Data.Size,
			data.Data.Color,
			id,
			data.Data.Is_tx,
		)
		ce(err, "update status")
		if nImages > 0 {
			_, err = tx.Exec(`DELETE FROM images_not_collected
				WHERE good_id = $1
				`,
				id,
			)
			ce(err, "delete images_not_collected entry")
			_, err = tx.Exec(`UPDATE goods
				SET images_collected = true
				WHERE good_id = $1
				`,
				id,
			)
			ce(err, "update goods.images_collected")
		} else {
			zeroImagesIdsLock.Lock()
			zeroImagesIds[id]++
			zeroImagesIdsLock.Unlock()
		}
		return
	}), "tx")
	return
}

func saveGoodImage(tx *sqlx.Tx, goodId int64, url string) (err error) {
	defer ct(&err)
	if url == "" {
		return
	}
	var imageId int64
	err = tx.Get(&imageId, `INSERT INTO images (good_id, url)
		VALUES ($1, $2)
		ON CONFLICT (good_id, url) DO NOTHING
		RETURNING image_id
		`,
		goodId,
		url,
	)
	ce(err, "insert image")
	var notHashed bool
	ce(tx.Get(&notHashed, `SELECT
		sha512_16k IS NULL
		FROM images
		WHERE image_id = $1
		`,
		imageId,
	), "get hash state")
	if notHashed {
		_, err = tx.Exec(`INSERT INTO not_hashed (image_id)
			VALUES ($1)
			`,
			imageId,
		)
		ce(err, "insert into not_hashed")
	}
	return
}
