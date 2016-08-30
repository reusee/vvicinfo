package main

import (
	"math"
)

func groupGoods() {
	goodIdToHashes := make(map[int32][]string)
	hashToGoodIds := make(map[string][]int32)
	rows, err := db.Query(`SELECT
		good_id, encode(sha512_16k, 'base64')
		FROM images
		WHERE sha512_16k IS NOT NULL
		AND (CASE WHEN length > 0 THEN length >= 50000 ELSE true END)
		`,
	)
	ce(err, "query")

	n := 0
	for rows.Next() {
		var goodId int32
		var hash string
		ce(rows.Scan(&goodId, &hash), "scan")
		goodIdToHashes[goodId] = append(goodIdToHashes[goodId], hash)
		hashToGoodIds[hash] = append(hashToGoodIds[hash], goodId)
		n++
		if n%10000 == 0 {
			pt("%d\n", n)
		}
	}
	ce(rows.Err(), "rows err")
	pt("images data loaded\n")

	goodIds := make(map[int32]struct{})
	rows, err = db.Query(`SELECT good_id
		FROM goods
		WHERE group_id IS NULL
		AND images_collected = true
		`,
	)
	ce(err, "query")
	for rows.Next() {
		var goodId int32
		ce(rows.Scan(&goodId), "scan")
		goodIds[goodId] = struct{}{}
	}
	ce(rows.Err(), "rows err")
	pt("good ids loaded\n")

	txCount := 0
	tx := db.MustBegin()
	markedCount := 0

check:

	var goodId int32
	for id := range goodIds {
		goodId = id
		break
	}
	hashes := goodIdToHashes[goodId]

	matches := make(map[int32]map[string]struct{})
	for _, hash := range hashes {
		rightIds := hashToGoodIds[hash]
		for _, rightId := range rightIds {
			if _, ok := matches[rightId]; !ok {
				matches[rightId] = make(map[string]struct{})
			}
			matches[rightId][hash] = struct{}{}
		}
	}

	has := false
	for rightId, hashSet := range matches {
		//TODO 去掉第二个条件
		if len(hashSet) >= 10 && math.Abs(float64(len(hashes)-len(hashSet))) < 5 {
			pt("%d %d %d %d - \n", goodId, len(hashes), rightId, len(hashSet), markedCount)
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
			markedCount++
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
		markedCount++
	}

	txCount++
	if txCount >= 64 {
		ce(tx.Commit(), "commit")
		tx = db.MustBegin()
		txCount = 0
	}

	goto check
}
