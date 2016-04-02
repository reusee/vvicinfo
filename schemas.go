package main

func initSchemas() {
	db.MustExec(`CREATE TABLE IF NOT EXISTS shops (
		shop_id INT PRIMARY KEY,
		name CHAR(128),
		update_at DATETIME,
		last_update_time INT(10)
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

	db.MustExec(`CREATE TABLE IF NOT EXISTS urls (
		url_id INT PRIMARY KEY AUTO_INCREMENT,
		url CHAR(255) NOT NULL,
		sha512 VARBINARY(64),
		UNIQUE INDEX url (url),
		INDEX sha512 (sha512)
	)
		ROW_FORMAT=COMPRESSED
	`)

	db.MustExec(`CREATE TABLE IF NOT EXISTS images (
		good_id INT UNSIGNED,
		url_id INT NOT NULL,
		UNIQUE INDEX good_image (good_id, url_id),
		INDEX url_id (url_id)
	)
		ROW_FORMAT=COMPRESSED
	`)
}
