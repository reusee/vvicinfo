package main

func initSchemas() {
	db.MustExec(`CREATE TABLE IF NOT EXISTS shops (
		shop_id INT PRIMARY KEY,
		name CHAR(128),
		update_at DATETIME
	)
		ROW_FORMAT=COMPRESSED
	`)
	db.MustExec(`CREATE TABLE IF NOT EXISTS goods (
		good_id INT UNSIGNED PRIMARY KEY,
		price DECIMAL(10, 2),
		shop_id INT,
		added_at CHAR(10),
		category INT,
		score DOUBLE,
		sort_score DOUBLE,
		title CHAR(255),
		status INT(1),
		INDEX shop_id (shop_id),
		INDEX added_at (added_at),
		INDEX category (category),
		INDEX status (status)
	)
		ROW_FORMAT=COMPRESSED
	`)
	db.MustExec(`CREATE TABLE IF NOT EXISTS images (
		good_id INT UNSIGNED,
		url CHAR(255),
		UNIQUE INDEX image_url (good_id, url)
	)
		ROW_FORMAT=COMPRESSED
	`)
}
