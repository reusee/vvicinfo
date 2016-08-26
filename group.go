package main

import (
	"github.com/lib/pq"
	"time"
)

func groupGoods() {
check:
	tx := db.MustBegin()

	// get good id
	var goodId int64
	var price float64
	var category int64
	ce(tx.QueryRow(`SELECT 
		good_id, price, category
		FROM goods
		WHERE group_id IS NULL 
		AND images_collected = true
		LIMIT 1`).Scan(&goodId, &price, &category), "get good id")
	pt("=> %d\n", goodId)

	// get good hashes
	var hashes pq.ByteaArray
	ce(tx.Select(&hashes, `SELECT
		sha512_16k
		FROM images i
		LEFT JOIN urls USING(url_id)
		WHERE
		i.good_id = $1
		`,
		goodId,
	), "select hashes")

	// make a map of good hashes
	type Hash [64]byte
	hashSet := make(map[Hash]struct{})
	for _, bs := range hashes {
		var hash Hash
		copy(hash[:], bs[:])
		hashSet[hash] = struct{}{}
	}

	// get may be the same good infos
	var infos []struct {
		GoodId   int64   `db:"good_id"`
		Hash     []byte  `db:"sha512_16k"`
		Price    float64 `db:"price"`
		Category int64   `db:"category"`
	}
	ce(tx.Select(&infos, `SELECT
		good_id, sha512_16k, price, category
		FROM images i
		LEFT JOIN urls USING(url_id)
		LEFT JOIN goods USING(good_id)
		WHERE
		sha512_16k = ANY($1)
		`,
		hashes,
	), "select infos")

	// make map of hashes
	hashSets := make(map[int64]map[Hash]struct{})
	goodPrices := make(map[int64]float64)
	goodCategories := make(map[int64]int64)
	for _, info := range infos {
		if _, ok := hashSets[info.GoodId]; !ok {
			hashSets[info.GoodId] = make(map[Hash]struct{})
		}
		var hash Hash
		copy(hash[:], info.Hash[:])
		hashSets[info.GoodId][hash] = struct{}{}
		goodPrices[info.GoodId] = info.Price
		goodCategories[info.GoodId] = info.Category
	}

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

	// check
	hashLen := len(hashSet)
	for rightId, hashes := range hashSets {
		n := 0
		for hash := range hashes {
			if _, ok := hashSet[hash]; ok {
				n++
			}
		}
		pt("-> %d %d %d\n", rightId, n, hashLen)
		if false ||
			(n == hashLen) ||
			(hashLen <= 6 && n >= 3) ||
			(n >= 13) ||
			(n >= 8 && hashLen <= 12) ||
			false { // 显然相同
			mark(rightId)

		} else if false ||
			(hashLen-n >= 12) ||
			(n <= 2 && hashLen >= 6) ||
			(n <= 5 && hashLen >= 20) ||
			(n <= 6 && hashLen >= 36) ||
			false { // 显然不同

		} else if false ||
			(n >= 8 && hashLen <= 22) ||
			(n == 3 && hashLen == 10) ||
			(n == 4 && hashLen == 18) ||
			(n == 5 && hashLen == 18) ||
			(n == 5 && hashLen == 17) ||
			false { // 可能相同，看看标题
			var similarity float64
			ce(tx.Get(&similarity, `SELECT similarity(
				(SELECT title FROM goods WHERE good_id = $1),
				(SELECT title FROM goods WHERE good_id = $2)
				)
				`,
				rightId,
				goodId,
			), "get similarity")
			if similarity >= 0.11 && goodCategories[goodId] == goodCategories[rightId] {
				mark(rightId)
			} else {
				time.Sleep(time.Second)
				panic("check this")
			}
		} else {
			time.Sleep(time.Second)
			panic("check this")
		}
	}

	_, err := tx.Exec(`UPDATE goods
		SET group_id = good_id
		WHERE good_id = $1
		`,
		goodId,
	)
	ce(err, "update goods")

	ce(tx.Commit(), "commit")

	goto check
}
