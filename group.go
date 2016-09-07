package main

import (
	"math"
	"runtime"
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
		if n%2000000 == 0 {
			runtime.GC()
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
	pt("%d goods to check\n", len(goodIds))

	txCount := 0
	tx := db.MustBegin()
	markedCount := 0

check:

	// 取一个good id来处理
	if len(goodIds) == 0 {
		tx.Commit()
		return
	}
	var goodId int32
	for id := range goodIds {
		goodId = id
		break
	}

	hashes := goodIdToHashes[goodId]
	if len(hashes) < 10 { // 图片数量少于10，不做染色
		_, err := tx.Exec(`UPDATE goods
				SET group_id = -1
				WHERE good_id = $1
				`,
			goodId,
		)
		ce(err, "update goods")
		delete(goodIds, goodId)
		markedCount++
		goto check
	}

	// 收集匹配的hash
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

	// 检查匹配的hash
	for rightId, matchSet := range matches {
		if len(matchSet) < 10 || math.Abs(float64(len(hashes)-len(goodIdToHashes[rightId]))) > 5 {
			continue
		}
		pt("%d %d %d %d %d\n", goodId, len(hashes), rightId, len(matchSet), markedCount)
		_, err := tx.Exec(`UPDATE goods
				SET group_id = $1
				WHERE good_id = $2
				`,
			goodId,
			rightId,
		)
		ce(err, "update goods")
		delete(goodIds, rightId)
		markedCount++
	}

	if markedCount%1000 == 0 {
		pt("marked %d\n", markedCount)
	}

	txCount++
	if txCount >= 128 {
		ce(tx.Commit(), "commit")
		tx = db.MustBegin()
		txCount = 0
	}

	goto check
}
