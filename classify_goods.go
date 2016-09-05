package main

import (
	"github.com/lib/pq"
)

func classifyGoods() {
	var skipGoodIds pq.Int64Array
	skipGoodIds = append(skipGoodIds, -1)

check:
	// select one good
	var goodId int64
	var groupId int64
	var shopId int64
	var internalId string
	ce(db.QueryRow(`SELECT
    good_id, group_id, shop_id, internal_id
    FROM goods
    LEFT JOIN shops USING (shop_id)
    WHERE 
    class_id IS NULL
    AND group_id > 0
    AND internal_id IS NOT NULL
    AND blacklisted = false
    AND NOT (good_id = ANY($1))
    LIMIT 1
    `,
		skipGoodIds,
	).Scan(&goodId, &groupId, &shopId, &internalId), "get good id")

	// get class ids by group_id or shop internal_id
	var classIds []int64
	ce(db.Select(&classIds, `SELECT
    DISTINCT class_id
    FROM goods
    WHERE
    (group_id = $1
    OR (shop_id = $2 AND internal_id = $3))
    AND class_id IS NOT NULL
    `,
		groupId,
		shopId,
		internalId,
	), "select class ids")

	if len(classIds) == 0 { // create new class
		tx := db.MustBegin()
		var classId int64
		ce(tx.Get(&classId, `INSERT INTO classes
      (images)
      VALUES (ARRAY(
        SELECT image_id FROM images
        WHERE good_id = $1)
      )
      RETURNING class_id
      `,
			goodId,
		), "insert class")
		_, err := tx.Exec(`UPDATE goods
      SET class_id = $1
      WHERE good_id = $2
      `,
			classId,
			goodId,
		)
		ce(err, "update good")
		ce(tx.Commit(), "commit")
		pt("%d %d\n", goodId, classId)
	} else if len(classIds) == 1 {
		_, err := db.Exec(`UPDATE goods
      SET class_id = $1
      WHERE good_id = $2
      `,
			classIds[0],
			goodId,
		)
		ce(err, "update good")
		pt("%d %d\n", goodId, classIds[0])
	} else {
		//TODO multiple class, fix this
		// 可能原因是同档口货号，但款式不同，档口的问题
		skipGoodIds = append(skipGoodIds, goodId)
	}

	goto check
}
