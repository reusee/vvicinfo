package main

import (
	"encoding/base64"
	"math"
	"runtime"
	"sync/atomic"
)

func groupGoods() {
	var mapReady, done int32
	goodIdToHashes := make(map[int32][]string)
	hashToGoodIds := make(map[string][]int32)
	go func() {
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
				if atomic.LoadInt32(&done) > 0 {
					return
				}
			}
			if n%2000000 == 0 {
				runtime.GC()
			}
		}
		ce(rows.Err(), "rows err")
		pt("images data loaded\n")
		atomic.StoreInt32(&mapReady, 1)
	}()

	goodIds := make(map[int32]struct{})
	rows, err := db.Query(`SELECT good_id
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
	dataIsReady := atomic.LoadInt32(&mapReady) > 0

	var goodId int32
	if len(goodIds) == 0 {
		tx.Commit()
		atomic.StoreInt32(&done, 1)
		return
	}
	for id := range goodIds {
		goodId = id
		break
	}

	var hashes []string
	if dataIsReady {
		hashes = goodIdToHashes[goodId]
	} else {
		err := db.Select(&hashes, `SELECT
			encode(sha512_16k, 'base64')
			FROM images
			WHERE good_id = $1
			`,
			goodId,
		)
		ce(err, "get hashes")
	}

	matches := make(map[int32]map[string]struct{})
	for _, hash := range hashes {
		var rightIds []int32
		if dataIsReady {
			rightIds = hashToGoodIds[hash]
		} else {
			bs, err := base64.StdEncoding.DecodeString(hash)
			ce(err, "decode hash")
			err = db.Select(&rightIds, `SELECT
				good_id
				FROM images
				WHERE
				sha512_16k = $1
				`,
				bs,
			)
			ce(err, "get ids")
		}
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
			pt("%d %d %d %d %d %v\n", goodId, len(hashes), rightId, len(hashSet), markedCount, dataIsReady)
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
			delete(goodIds, rightId)
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
		delete(goodIds, goodId)
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
