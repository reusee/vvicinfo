package main

import (
	//"github.com/jmoiron/sqlx"
	//_ "github.com/lib/pq"
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
	ce(db.Select(&groupRows, `SELECT 
		group_id, hashes, title
		FROM groups`), "select group infos")
	groupIdToHashSet := make(map[GroupId]map[Hash]struct{})
	hashToGroupIdSet := make(map[Hash]map[GroupId]struct{})
	for _, info := range groupRows {
		for _, hash := range info.Hashes {
			hash := Hash(hash)
			if _, ok := groupIdToHashSet[info.GroupId]; !ok {
				groupIdToHashSet[info.GroupId] = make(map[Hash]struct{})
			}
			groupIdToHashSet[info.GroupId][hash] = struct{}{}
			if _, ok := hashToGroupIdSet[hash]; !ok {
				hashToGroupIdSet[hash] = make(map[GroupId]struct{})
			}
			hashToGroupIdSet[hash][info.GroupId] = struct{}{}
		}
	}
	pt("group infos loaded\n")

select_goods:
	tx := db.MustBegin()

	var goodIds Int64Array
	err := tx.Select(&goodIds, `SELECT good_id
			FROM goods
			WHERE added_at >= $1
			--AND status > 0
			AND group_id IS NULL
			ORDER BY good_id DESC
			LIMIT 256
			`,
		time.Now().Add(-time.Hour*24*120).Format("2006-01-02"),
	)
	ce(err, "select goods")
	pt("select %d goods\n", len(goodIds))
	if len(goodIds) == 0 {
		return
	}

	//XXX manually
	//var err error
	//goodIds := Int64Array{
	//}

	//XXX from vvic db
	//lalaDB, err := sqlx.Connect("postgres", "host=reus.mobi user=reus dbname=lala connect_timeout=60")
	//ce(err, "connect to lala db")
	//var goodIdsFromLala Int64Array
	//ce(lalaDB.Select(&goodIdsFromLala, `SELECT vvic_id FROM items`), "select good ids from lala db")
	//var goodIds Int64Array
	//ce(db.Select(&goodIds, `SELECT good_id FROM goods
	//	WHERE good_id = ANY($1)
	//	AND group_id IS NULL
	//	`,
	//	goodIdsFromLala,
	//), "select good ids")
	//pt("%d good ids\n", len(goodIds))
	//if len(goodIds) == 0 {
	//	return
	//}

	var infos []struct {
		GoodId GoodId `db:"good_id"`
		Hash   string `db:"hash"`
	}
	err = tx.Select(&infos, `SELECT i.good_id, encode(sha512_16k, 'base64') AS hash
		FROM images i 
		LEFT JOIN urls u ON u.url_id = i.url_id
		WHERE i.good_id = ANY($1)
		AND sha512_16k IS NOT NULL
		`,
		goodIds,
	)
	ce(err, "select hashes")
	goodIdToHashSet := make(map[GoodId]map[Hash]struct{})
	for _, info := range infos {
		if _, ok := goodIdToHashSet[info.GoodId]; !ok {
			goodIdToHashSet[info.GoodId] = make(map[Hash]struct{})
		}
		goodIdToHashSet[info.GoodId][Hash(info.Hash)] = struct{}{}
	}
	pt("select %d rows of infos\n", len(infos))

loop_goods:
	for _, goodId := range goodIds {
		goodId := GoodId(goodId)
		candidateGroupIdSet := make(map[GroupId]struct{})
		for hash := range goodIdToHashSet[goodId] {
			if groupIdSet, ok := hashToGroupIdSet[hash]; ok {
				for groupId := range groupIdSet {
					candidateGroupIdSet[groupId] = struct{}{}
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
				if similarity < 0.3 && count < 10 {
					// 有10个图相同的话，就不管标题了
					// 如果发现有商品的小图用量大于10,那就提高
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
				groupIdToHashSet[newGroupId] = make(map[Hash]struct{})
			}
			groupIdToHashSet[newGroupId][hash] = struct{}{}
			if _, ok := hashToGroupIdSet[hash]; !ok {
				hashToGroupIdSet[hash] = make(map[GroupId]struct{})
			}
			hashToGroupIdSet[hash][newGroupId] = struct{}{}
		}
		_, err := tx.Exec(`UPDATE goods
				SET group_id = $1
				WHERE good_id = $2
				`,
			newGroupId,
			goodId,
		)
		ce(err, "update goods")

	}

	ce(tx.Commit(), "commit")

	goto select_goods

}
