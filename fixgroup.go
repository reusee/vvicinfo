package main

func groupByInternalId() {
	tx := db.MustBegin()
	defer func() {
		ce(tx.Commit(), "commit")
	}()

	rows, err := tx.Query(`SELECT
		good_id, group_id, shop_id, internal_id
		FROM goods
		WHERE internal_id IS NOT NULL
		AND group_id IS NOT NULL
		ORDER BY good_id DESC
		`,
	)
	ce(err, "select infos")

	type ShopId int
	type GoodId int64
	type GroupId int
	type Info struct {
		GroupId GroupId
		GoodId  GoodId
	}
	m := make(map[ShopId]map[string]Info)

	for rows.Next() {
		var goodId GoodId
		var groupId GroupId
		var shopId ShopId
		var internalId string
		ce(rows.Scan(&goodId, &groupId, &shopId, &internalId), "scan")
		if internalId == "" {
			continue
		}
		if _, ok := m[shopId]; !ok {
			m[shopId] = make(map[string]Info)
		}
		if info, ok := m[shopId][internalId]; ok {
			if info.GroupId != groupId { // fix
				pt("%s: %d %d -> %d %d\n", internalId, goodId, groupId, info.GoodId, info.GroupId)
				_, err := tx.Exec(`UPDATE goods
					SET group_id = $1
					WHERE good_id = $2
					`,
					info.GroupId,
					goodId,
				)
				ce(err, "update goods")
			}
		} else {
			m[shopId][internalId] = Info{
				GroupId: groupId,
				GoodId:  goodId,
			}
		}
	}
	ce(rows.Err(), "rows err")
}
