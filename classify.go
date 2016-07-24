package main

import (
	"math"
	"time"
)

func classifyGoods() {
	// load existing group infos
	type GroupInfo struct {
		GroupId int         `db:"group_id"`
		Hashes  StringArray `db:"hashes"`
	}
	var groupRows []GroupInfo
	ce(db.Select(&groupRows, `SELECT * FROM groups`), "select group infos")
	groups := make(map[int]GroupInfo)
	hashGroups := make(map[string]map[int]struct{})
	for _, info := range groupRows {
		groups[info.GroupId] = info
		for _, hash := range info.Hashes {
			if _, ok := hashGroups[hash]; !ok {
				hashGroups[hash] = make(map[int]struct{})
			}
			hashGroups[hash][info.GroupId] = struct{}{}
		}
	}
	pt("group infos loaded\n")

select_goods:
	var goodIds Int64Array
	err := db.Select(&goodIds, `SELECT good_id 
		FROM goods
		WHERE title LIKE $1
		AND added_at >= $2
		AND status > 0
		AND group_id IS NULL
		LIMIT 128
		`,
		"%牛仔%",
		time.Now().Add(-time.Hour*24*45).Format("2006-01-02"),
	)
	ce(err, "select goods")
	pt("select %d goods\n", len(goodIds))
	if len(goodIds) == 0 {
		return
	}

	goodHashes := make(map[int64][]string)
	hashGoods := make(map[string][]int64)
	var infos []struct {
		GoodId int64  `db:"good_id"`
		Hash   string `db:"hash"`
	}
	err = db.Select(&infos, `SELECT g.good_id, encode(sha512_16k, 'hex') AS hash
		FROM goods g
		LEFT JOIN images i ON g.good_id = i.good_id
		LEFT JOIN urls u ON u.url_id = i.url_id
		WHERE g.good_id = ANY($1)
		`,
		goodIds,
	)
	ce(err, "select hashes")
	for _, info := range infos {
		goodHashes[info.GoodId] = append(goodHashes[info.GoodId], info.Hash)
		hashGoods[info.Hash] = append(hashGoods[info.Hash], info.GoodId)
	}
	pt("select %d rows of infos\n", len(infos))

loop_goods:
	for _, goodId := range goodIds {
		for _, hash := range goodHashes[goodId] {
			if groupSet, ok := hashGroups[hash]; ok {
				for groupId := range groupSet {
					// 判断是否同一组，是的就加入该组
					count := 0
					for _, hash := range groups[groupId].Hashes {
						for _, goodHash := range goodHashes[goodId] {
							if hash == goodHash {
								count++
							}
						}
					}
					if math.Abs(float64(len(groups[groupId].Hashes)-count)) <= 3 {
						//same group
						_, err := db.Exec(`UPDATE goods 
							SET group_id = $1
							WHERE good_id = $2
							`,
							groupId,
							goodId,
						)
						ce(err, "update goods")
						continue loop_goods
					}
				}
			}
		}

		// new group or ignore
		if len(goodHashes[goodId]) > 5 {
			// new group
			row := db.QueryRow(`INSERT INTO groups
								(hashes) VALUES ($1)
								RETURNING group_id
								`,
				StringArray(goodHashes[goodId]),
			)
			ce(err, "insert new group")
			var newGroupId int
			ce(row.Scan(&newGroupId), "scan")
			groups[newGroupId] = GroupInfo{
				GroupId: newGroupId,
				Hashes:  StringArray(goodHashes[goodId]),
			}
			for _, hash := range goodHashes[goodId] {
				if _, ok := hashGroups[hash]; !ok {
					hashGroups[hash] = make(map[int]struct{})
				}
				hashGroups[hash][newGroupId] = struct{}{}
			}
			_, err := db.Exec(`UPDATE goods
				SET group_id = $1
				WHERE good_id = $2
				`,
				newGroupId,
				goodId,
			)
			ce(err, "update goods")
		} else {
			// ignore this item
			_, err := db.Exec(`UPDATE goods
								SET group_id = 0
								WHERE good_id = $1
								`,
				goodId,
			)
			ce(err, "update goods")
		}

	}

	goto select_goods

	time.Sleep(time.Second)
}
