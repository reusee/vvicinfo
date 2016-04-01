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
		price DECIMAL(10, 2) NOT NULL,
		shop_id INT NOT NULL,
		added_at CHAR(10),
		category INT NOT NULL,
		score DOUBLE,
		sort_score DOUBLE,
		title CHAR(255),
		status INT(1) NOT NULL,
		INDEX shop_id (shop_id),
		INDEX added_at (added_at),
		INDEX category (category),
		INDEX status (status)
	)
		ROW_FORMAT=COMPRESSED
	`)
	db.MustExec(`CREATE TABLE IF NOT EXISTS images (
		good_id INT UNSIGNED,
		url CHAR(255) NOT NULL,
		sha512 VARBINARY(64),
		UNIQUE INDEX good_image (good_id, url),
		INDEX sha512 (sha512),
		INDEX good_id (good_id)
	)
		ROW_FORMAT=COMPRESSED
	`)
}
