package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
)

var (
	db *sqlx.DB
	pt = fmt.Printf
)

func init() {
	var err error
	db, err = sqlx.Connect("postgres", "user=reus dbname=vvic sslmode=disable")
	ce(err, "connect to db")
}

func main() {
	err := selectMostManufacturedItems(os.Args[1], os.Args[2:])
	ce(err, "selectMostManufacturedItems")
}

func selectMostManufacturedItems(dateFrom string, keywords []string) (err error) {
	defer ct(&err)
	pt("// === from %s keywords %v ===\n", dateFrom, keywords)
	likeStmt := "("
	for i, keyword := range keywords {
		if i > 0 {
			likeStmt += " AND "
		}
		likeStmt += "a.title LIKE '%" + keyword + "%'"
	}
	likeStmt += ")"
	rows, err := db.Queryx(`SELECT 
		a.good_id, a.shop_id, c.sha512_16k
		FROM goods a
		LEFT JOIN images b ON a.good_id = b.good_id
		LEFT JOIN urls c ON b.url_id = c.url_id
		LEFT JOIN shops d ON a.shop_id = d.shop_id
		WHERE
		a.added_at > $1
		AND `+likeStmt+`
		AND a.status = 1
		`,
		dateFrom,
	)
	ce(err, "select items")
	n := 0
	hashShopStats := make(map[string]map[int64]bool)
	hashGoodStats := make(map[string]map[int64]bool)
	for rows.Next() {
		var row struct {
			GoodId int64  `db:"good_id"`
			ShopId int64  `db:"shop_id"`
			Hash   []byte `db:"sha512_16k"`
		}
		ce(rows.StructScan(&row), "scan")
		if len(row.Hash) == 0 {
			continue
		}

		key := string(row.Hash)
		if _, ok := hashShopStats[key]; !ok {
			hashShopStats[key] = make(map[int64]bool)
		}
		hashShopStats[key][row.ShopId] = true
		if _, ok := hashGoodStats[key]; !ok {
			hashGoodStats[key] = make(map[int64]bool)
		}
		hashGoodStats[key][row.GoodId] = true

		n++
		//if n%300 == 0 {
		//	pt("%d\n", n)
		//}
	}
	ce(rows.Err(), "rows err")

	// sort
	keys := Strs([]string{})
	for key := range hashShopStats {
		keys = append(keys, key)
	}
	keys.Sort(func(a, b string) bool {
		return len(hashShopStats[a]) > len(hashShopStats[b])
	})

	// dedup
	var selectedKeys []string
	var lastKey string
	for i, key := range keys {
		if i == 0 {
			selectedKeys = append(selectedKeys, key)
			lastKey = key
			continue
		}
		n := 0
		for goodId := range hashGoodStats[lastKey] {
			if _, ok := hashGoodStats[key][goodId]; ok {
				n++
			}
		}
		similarity := float64(n) / float64(len(hashGoodStats[lastKey]))
		if len(hashGoodStats[lastKey]) > 8 && similarity > 0.7 { // same good set
			continue
		}
		selectedKeys = append(selectedKeys, key)
		lastKey = key
	}

	type GoodInfo struct {
		GoodId int64   `db:"good_id" json:"good_id"`
		Price  float64 `json:"price"`
	}
	type Entry struct {
		Images []string   `json:"images"`
		Goods  []GoodInfo `json:"goods"`
	}
	var entries []Entry

	// collect
	goodOccurCount := make(map[int64]int)
	for i, key := range selectedKeys {
		//if i > 1000 {
		//	break
		//}
		_ = i
		if len(hashShopStats[key]) < 4 {
			break
		}
		var entry Entry
		var imageUrls []string
		err := db.Select(&imageUrls, `SELECT url FROM urls
			WHERE sha512_16k = $1
			LIMIT 1`,
			key,
		)
		ce(err, "select images")
		entry.Images = imageUrls

		var goodIds []int64
		for goodId := range hashGoodStats[key] {
			if n, ok := goodOccurCount[goodId]; !ok { // new good id
				goodIds = append(goodIds, goodId)
				goodOccurCount[goodId] = 1
			} else {
				if n < 2 {
					goodIds = append(goodIds, goodId)
					goodOccurCount[goodId]++
				}
				// skip n >= 2
			}
		}
		if len(goodIds) == 0 {
			continue
		}
		query, args, err := sqlx.In(`SELECT a.good_id, a.price
			FROM goods a
			LEFT JOIN shops b
			ON a.shop_id = b.shop_id
			WHERE good_id IN (?)
			ORDER BY good_id DESC`,
			goodIds)
		ce(err, "in query")
		query = sqlx.Rebind(sqlx.DOLLAR, query)
		var goodInfos []GoodInfo
		err = db.Select(&goodInfos, query, args...)
		ce(err, "select")
		entry.Goods = goodInfos

		entries = append(entries, entry)
	}

	// output
	pt("window.data = ")
	j, err := json.Marshal(entries)
	ce(err, "marshal")
	buf := new(bytes.Buffer)
	ce(json.Indent(buf, j, "", "    "), "indent")
	pt("%s\n", buf.Bytes())

	return
}
