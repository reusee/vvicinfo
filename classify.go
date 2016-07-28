package main

import (
	"time"
)

func classifyGoods() {
	defer time.Sleep(time.Second)

	type GroupId int
	type Hash string
	type GoodId int64

	// load existing group infos
	type GroupInfo struct {
		GroupId GroupId     `db:"group_id"`
		Hashes  StringArray `db:"hashes"`
		Title   string      `db:"title"`
	}
	var groupRows []GroupInfo
	ce(db.Select(&groupRows, `SELECT * FROM groups`), "select group infos")
	groupIdToHashSet := make(map[GroupId]map[Hash]bool)
	hashToGroupIdSet := make(map[Hash]map[GroupId]bool)
	for _, info := range groupRows {
		for _, hash := range info.Hashes {
			hash := Hash(hash)
			if _, ok := groupIdToHashSet[info.GroupId]; !ok {
				groupIdToHashSet[info.GroupId] = make(map[Hash]bool)
			}
			groupIdToHashSet[info.GroupId][hash] = true
			if _, ok := hashToGroupIdSet[hash]; !ok {
				hashToGroupIdSet[hash] = make(map[GroupId]bool)
			}
			hashToGroupIdSet[hash][info.GroupId] = true
		}
	}
	pt("group infos loaded\n")

select_goods:
	tx := db.MustBegin()

	var goodIds Int64Array
	err := tx.Select(&goodIds, `SELECT good_id
			FROM goods
			WHERE added_at >= $1
			AND status > 0
			AND group_id IS NULL
			LIMIT 256
			`,
		time.Now().Add(-time.Hour*24*45).Format("2006-01-02"),
	)
	ce(err, "select goods")
	pt("select %d goods\n", len(goodIds))
	if len(goodIds) == 0 {
		return
	}

	//XXX debug
	//var err error
	//goodIds := Int64Array{
	//	2398924,
	//}

	var infos []struct {
		GoodId GoodId `db:"good_id"`
		Hash   string `db:"hash"`
	}
	err = tx.Select(&infos, `SELECT i.good_id, encode(sha512_16k, 'hex') AS hash
		FROM images i 
		LEFT JOIN urls u ON u.url_id = i.url_id
		WHERE i.good_id = ANY($1)
		`,
		goodIds,
	)
	ce(err, "select hashes")
	goodIdToHashSet := make(map[GoodId]map[Hash]bool)
	for _, info := range infos {
		if _, ok := goodIdToHashSet[info.GoodId]; !ok {
			goodIdToHashSet[info.GoodId] = make(map[Hash]bool)
		}
		goodIdToHashSet[info.GoodId][Hash(info.Hash)] = true
	}
	pt("select %d rows of infos\n", len(infos))

loop_goods:
	for _, goodId := range goodIds {
		goodId := GoodId(goodId)
		candidateGroupIdSet := make(map[GroupId]bool)
		for hash := range goodIdToHashSet[goodId] {
			if groupIdSet, ok := hashToGroupIdSet[hash]; ok {
				for groupId := range groupIdSet {
					candidateGroupIdSet[groupId] = true
				}
			}
		}

		for groupId := range candidateGroupIdSet {
			count := 0
			total := 0
			for hash := range groupIdToHashSet[groupId] {
				if _, ok := goodIdToHashSet[goodId][hash]; ok {
					count++
				}
				total++
			}
			if total-count <= 3 && count > 5 {
				var similarity float64
				ce(tx.Get(&similarity, `SELECT similarity(
					(SELECT title FROM groups WHERE group_id = $1),
					(SELECT title FROM goods WHERE good_id = $2)
				)`,
					groupId,
					goodId,
				), "get similarity")
				if similarity < 0.3 && count < 15 {
					// 有15个图相同的话，就不管标题了
					// 如果发现有商品的小图用量大于15,那就提高
					continue
				}
				//same group
				_, err := tx.Exec(`UPDATE goods 
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

		// new group or ignore
		if len(goodIdToHashSet[goodId]) > 5 {
			// new group
			var groupHashes StringArray
			for hash := range goodIdToHashSet[goodId] {
				groupHashes = append(groupHashes, string(hash))
			}
			row := tx.QueryRow(`INSERT INTO groups
				(hashes, title) 
				VALUES ($1, (SELECT title FROM goods WHERE good_id = $2))
				RETURNING group_id
				`,
				groupHashes,
				goodId,
			)
			ce(err, "insert new group")
			var newGroupId GroupId
			ce(row.Scan(&newGroupId), "scan")
			for hash := range goodIdToHashSet[goodId] {
				if _, ok := groupIdToHashSet[newGroupId]; !ok {
					groupIdToHashSet[newGroupId] = make(map[Hash]bool)
				}
				groupIdToHashSet[newGroupId][hash] = true
				if _, ok := hashToGroupIdSet[hash]; !ok {
					hashToGroupIdSet[hash] = make(map[GroupId]bool)
				}
				hashToGroupIdSet[hash][newGroupId] = true
			}
			_, err := tx.Exec(`UPDATE goods
				SET group_id = $1
				WHERE good_id = $2
				`,
				newGroupId,
				goodId,
			)
			ce(err, "update goods")
		} else {
			// ignore this item
			_, err := tx.Exec(`UPDATE goods
								SET group_id = 0
								WHERE good_id = $1
								`,
				goodId,
			)
			ce(err, "update goods")
		}

	}

	ce(tx.Commit(), "commit")

	goto select_goods

}
