package main

import (
	"encoding/json"
	"fmt"
	"github.com/PuerkitoBio/goquery"
	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
)

var zeroImagesIds = make(map[int64]int)
var zeroImagesIdsLock = new(sync.Mutex)

func collectGoods() {
	var total uint64
do:
	pt("select good ids to fetch images\n")
	var ids, filtered []int64
	err := db.Select(&ids, `SELECT good_id FROM images_not_collected ORDER BY good_id DESC`)
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
			ce(err, "collect detail page")
			pt("%10d / %10d / %10d collected, %10d images, id %10d\n",
				i, l, atomic.AddUint64(&total, 1), n, id)
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
			Imgs   string // 封面图片
			Desc   string // 描述html
			Status int    // 上下架状态
			Size   string // 尺码
			Color  string // 颜色
			Is_tx  int    // 退现
			Attrs  string // 属性
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

	tx := db.MustBegin()

	nImages := 0
	var coverImageIds []int64
	for _, imgPath := range strings.Split(data.Data.Imgs, ",") {
		if imgPath == "" {
			continue
		}
		if !strings.HasPrefix(imgPath, "http:") {
			imgPath = "http:" + imgPath
		}
		imgId, err := saveGoodImage(tx, id, imgPath)
		ce(err, "save image")
		coverImageIds = append(coverImageIds, imgId)
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
		_, err := saveGoodImage(tx, id, imgSrc)
		ce(err, "save image")
		n++
		nImages++
	})
	_, err = tx.Exec(`UPDATE goods 
			SET status = $1, sizes = $2, colors = $3, tuixian = $5, attributes = $6, description = $7,
			cover_image_ids = $8
			WHERE good_id = $4`,
		data.Data.Status,
		data.Data.Size,
		data.Data.Color,
		id,
		data.Data.Is_tx,

		data.Data.Attrs,
		data.Data.Desc,
		pq.Int64Array(coverImageIds),
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

	ce(tx.Commit(), "commit")

	return
}

func saveGoodImage(tx *sqlx.Tx, goodId int64, url string) (imageId int64, err error) {
	if url == "" {
		return 0, me(nil, "bad url")
	}
	err = tx.Get(&imageId, `INSERT INTO images (good_id, url)
		VALUES ($1, $2)
		ON CONFLICT (good_id, url) DO 
		UPDATE SET image_id = images.image_id
		RETURNING image_id
		`,
		goodId,
		url,
	)
	if err != nil {
		return 0, me(err, "insert image")
	}
	_, err = tx.Exec(`INSERT INTO image_vars
		(image_id)
		VALUES ($1)
		ON CONFLICT (image_id) DO NOTHING
		`,
		imageId,
	)
	if err != nil {
		return 0, me(err, "insert image vars")
	}
	return imageId, nil
}
