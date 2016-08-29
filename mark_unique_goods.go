package main

func markUniqueGoods() {
	tx := db.MustBegin()
	defer func() {
		ce(tx.Commit(), "commit")
	}()

	// 不需要，一次独家，永远独家
	//_, err := tx.Exec(`UPDATE goods SET
	//	is_unique = false
	//	WHERE
	//	is_unique = true
	//	`,
	//)
	//ce(err, "set all is_unique to false")

	_, err := tx.Exec(`UPDATE goods SET 
		is_unique = true
		WHERE group_id = ANY(
			SELECT group_id FROM (
				SELECT COUNT(DISTINCT internal_id) AS n, 
				group_id 
				FROM goods 
				GROUP BY group_id
			) t0 
			WHERE n = 1
		)
		`,
	)
	ce(err, "update is_unique")
}
