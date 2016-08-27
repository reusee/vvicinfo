package main

import (
	"github.com/lib/pq"
)

func groupGoods() {
	// load image and urls infos
	hashToUrlId := make(map[string]int64)
	rows, err := db.Query(`SELECT url_id, encode(sha512_16k, 'base64') FROM urls
		WHERE sha512_16k IS NOT NULL
		--ORDER BY url_id DESC
		--LIMIT 50000 -- DEBUG
		`)
	ce(err, "query")
	n := 0
	for rows.Next() {
		var urlId int64
		var hash string
		ce(rows.Scan(&urlId, &hash), "scan")
		hashToUrlId[hash] = urlId
		n++
		if n%10000 == 0 {
			pt("%d\n", n)
		}
	}
	ce(rows.Err(), "rows err")
	pt("urls loaded\n")

	urlIdToGoodIds := make(map[int64][]int64)
	rows, err = db.Query(`SELECT good_id, url_id FROM images
		--ORDER BY image_id DESC
		--LIMIT 50000 -- DEBUG
		`)
	ce(err, "query")
	n = 0
	for rows.Next() {
		var goodId, urlId int64
		ce(rows.Scan(&goodId, &urlId), "scan")
		urlIdToGoodIds[urlId] = append(urlIdToGoodIds[urlId], goodId)
		n++
		if n%10000 == 0 {
			pt("%d\n", n)
		}
	}
	ce(rows.Err(), "rows err")
	pt("images loaded\n")

	txCount := 0
	tx := db.MustBegin()

check:

	// get good id
	var goodId int64
	var price float64
	var category int64
	ce(tx.QueryRow(`SELECT 
		good_id, price, category
		FROM goods
		WHERE group_id IS NULL 
		AND images_collected = true
		ORDER BY good_id DESC
		LIMIT 1`).Scan(&goodId, &price, &category), "get good id")

	// get good hashes
	var hashes pq.StringArray
	ce(tx.Select(&hashes, `SELECT
		encode(sha512_16k, 'base64')
		FROM images i
		LEFT JOIN urls USING(url_id)
		WHERE
		i.good_id = $1
		`,
		goodId,
	), "select hashes")

	// mark as same
	mark := func(rightId int64) {
		_, err := tx.Exec(`UPDATE goods
				SET group_id = $1
				WHERE good_id = $2
				`,
			goodId,
			rightId,
		)
		ce(err, "update goods")
	}

	// stat
	matches := make(map[int64]int)
	for _, hash := range hashes {
		for _, rightId := range urlIdToGoodIds[hashToUrlId[hash]] {
			matches[rightId]++
		}
	}
	has := false
	for rightId, n := range matches {
		if n >= 10 {
			pt("-> %d %d\n", goodId, rightId)
			// 不到的话就算了吧
			has = true
			mark(rightId)
		}
	}
	if has {
		//mark(goodId)
	} else {
		_, err := tx.Exec(`UPDATE goods
				SET group_id = -1
				WHERE good_id = $1
				`,
			goodId,
		)
		ce(err, "update goods")
	}

	txCount++
	if txCount == 256 {
		ce(tx.Commit(), "commit")
		tx = db.MustBegin()
	}

	goto check
}
