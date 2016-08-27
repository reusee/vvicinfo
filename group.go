package main

import (
	"github.com/lib/pq"
)

func groupGoods() {
	// load image and urls infos
	hashToUrlIds := make(map[string][]int64)
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
		hashToUrlIds[hash] = append(hashToUrlIds[hash], urlId)
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
	ce(tx.Get(&goodId, `SELECT 
		good_id
		FROM goods
		WHERE group_id IS NULL 
		AND images_collected = true
		ORDER BY good_id DESC
		LIMIT 1`), "get good id")

	// get good hashes
	var hs pq.StringArray
	ce(tx.Select(&hs, `SELECT
		encode(sha512_16k, 'base64')
		FROM images i
		LEFT JOIN urls USING(url_id)
		WHERE
		i.good_id = $1
		`,
		goodId,
	), "select hashes")
	hashes := make(map[string]struct{})
	for _, h := range hs {
		hashes[h] = struct{}{}
	}

	// stat
	matches := make(map[int64]map[string]struct{})
	for hash := range hashes {
		urlIds := hashToUrlIds[hash]
		for _, urlId := range urlIds {
			for _, rightId := range urlIdToGoodIds[urlId] {
				if _, ok := matches[rightId]; !ok {
					matches[rightId] = make(map[string]struct{})
				}
				matches[rightId][hash] = struct{}{}
			}
		}
	}
	has := false
	for rightId, hashSet := range matches {
		if len(hashSet) >= 10 {
			pt("%d %d %d %d\n", goodId, len(hashes), rightId, len(hashSet))
			// 不到的话就算了吧
			has = true
			_, err := tx.Exec(`UPDATE goods
				SET group_id = $1
				WHERE good_id = $2
				`,
				goodId,
				rightId,
			)
			ce(err, "update goods")
		}
	}
	if !has {
		_, err := tx.Exec(`UPDATE goods
				SET group_id = -1
				WHERE good_id = $1
				`,
			goodId,
		)
		ce(err, "update goods")
	}

	txCount++
	if txCount >= 64 {
		ce(tx.Commit(), "commit")
		tx = db.MustBegin()
		txCount = 0
	}

	goto check
}
