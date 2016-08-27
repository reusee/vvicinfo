package main

import (
	//"bufio"
	"github.com/lib/pq"
	//"os/exec"
	//"strings"
)

func groupGoods() {
	tx := db.MustBegin()

	//cmd := exec.Command("psql", "-h", "localhost", "vvic", "-c", `
	//	copy (select url_id, encode(sha512_16k, 'base64') from urls) to stdout
	//	with delimiter '|'`)
	//stdout, err := cmd.StdoutPipe()
	//ce(err, "get stdout")
	//cmd.Start()
	//r := bufio.NewReaderSize(stdout, 1024*1024*32)
	//scanner := bufio.NewScanner(r)
	//hashToUrlId := make(map[string]string)
	//n := 0
	//for scanner.Scan() {
	//	parts := strings.Split(scanner.Text(), "|")
	//	hashToUrlId[parts[1]] = parts[0]
	//	n++
	//	if n%1000 == 0 {
	//		pt("%d\n", n)
	//	}
	//}
	//ce(scanner.Err(), "scan error")

	// load image and urls infos
	hashToUrlId := make(map[string]int64)
	rows, err := db.Query(`SELECT url_id, encode(sha512_16k, 'base64') FROM urls
		WHERE sha512_16k IS NOT NULL
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
	rows, err = db.Query(`SELECT good_id, url_id FROM images`)
	ce(err, "query")
	n = 0
	for rows.Next() {
		var urlId, goodId int64
		ce(rows.Scan(&urlId, &goodId), "scan")
		urlIdToGoodIds[urlId] = append(urlIdToGoodIds[urlId], goodId)
		n++
		if n%10000 == 0 {
			pt("%d\n", n)
		}
	}
	ce(rows.Err(), "rows err")
	pt("images loaded\n")

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
	pt("=> %d\n", goodId)

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
	for rightId, n := range matches {
		pt("-> %d\n", rightId)
		if n >= 10 {
			mark(rightId)
		}
	}

	// get may be the same good infos
	//var infos []struct {
	//	GoodId   int64   `db:"good_id"`
	//	Hash     []byte  `db:"sha512_16k"`
	//	Price    float64 `db:"price"`
	//	Category int64   `db:"category"`
	//}
	//ce(tx.Select(&infos, `SELECT
	//	good_id, sha512_16k, price, category
	//	FROM images i
	//	LEFT JOIN urls USING(url_id)
	//	LEFT JOIN goods USING(good_id)
	//	WHERE
	//	sha512_16k = ANY($1)
	//	`,
	//	hashes,
	//), "select infos")

	//// make map of hashes
	//hashSets := make(map[int64]map[Hash]struct{})
	//goodPrices := make(map[int64]float64)
	//goodCategories := make(map[int64]int64)
	//for _, info := range infos {
	//	if _, ok := hashSets[info.GoodId]; !ok {
	//		hashSets[info.GoodId] = make(map[Hash]struct{})
	//	}
	//	var hash Hash
	//	copy(hash[:], info.Hash[:])
	//	hashSets[info.GoodId][hash] = struct{}{}
	//	goodPrices[info.GoodId] = info.Price
	//	goodCategories[info.GoodId] = info.Category
	//}

	// check
	//hashLen := len(hashSet)
	//for rightId, hashes := range hashSets {
	//	n := 0
	//	for hash := range hashes {
	//		if _, ok := hashSet[hash]; ok {
	//			n++
	//		}
	//	}
	//	pt("-> %d %d %d\n", rightId, n, hashLen)
	//	if false ||
	//		(n == hashLen) ||
	//		(hashLen <= 6 && n >= 3) ||
	//		(n >= 13) ||
	//		(n >= 8 && hashLen <= 12) ||
	//		false { // 显然相同
	//		mark(rightId)

	//	} else if false ||
	//		(hashLen-n >= 12) ||
	//		(n <= 2 && hashLen >= 6) ||
	//		(n <= 5 && hashLen >= 20) ||
	//		(n <= 6 && hashLen >= 36) ||
	//		false { // 显然不同

	//	} else if false ||
	//		(n >= 8 && hashLen <= 22) ||
	//		(n == 3 && hashLen == 10) ||
	//		(n == 4 && hashLen == 18) ||
	//		(n == 5 && hashLen == 18) ||
	//		(n == 5 && hashLen == 17) ||
	//		false { // 可能相同，看看标题
	//		var similarity float64
	//		ce(tx.Get(&similarity, `SELECT similarity(
	//			(SELECT title FROM goods WHERE good_id = $1),
	//			(SELECT title FROM goods WHERE good_id = $2)
	//			)
	//			`,
	//			rightId,
	//			goodId,
	//		), "get similarity")
	//		if similarity >= 0.11 && goodCategories[goodId] == goodCategories[rightId] {
	//			mark(rightId)
	//		} else {
	//			time.Sleep(time.Second)
	//			panic("check this")
	//		}
	//	} else {
	//		time.Sleep(time.Second)
	//		panic("check this")
	//	}
	//}

	_, err = tx.Exec(`UPDATE goods
		SET group_id = good_id
		WHERE good_id = $1
		`,
		goodId,
	)
	ce(err, "update goods")

	ce(tx.Commit(), "commit")

	goto check
}
