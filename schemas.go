package main

func initSchemas() {
	db.MustExec(`CREATE TABLE IF NOT EXISTS ` + prefix + `_shops (
		shop_id INT PRIMARY KEY,
		name CHAR(128),
		update_at DATETIME
	)`)
	db.MustExec(`CREATE TABLE IF NOT EXISTS ` + prefix + `_goods (
		good_id INT UNSIGNED PRIMARY KEY,
		price DECIMAL(10, 2),
		shop_id INT,
		added_at CHAR(10),
		category INT,
		score DOUBLE,
		sort_score DOUBLE,
		title CHAR(255),
		INDEX shop_id (shop_id),
		INDEX added_at (added_at),
		INDEX category (category)
	)`)
}
