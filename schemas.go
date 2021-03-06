package main

import (
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
)

var db *sqlx.DB

func init() {
	var err error
	db, err = sqlx.Connect("postgres", "user=reus dbname=vvic sslmode=disable")
	ce(err, "connect to db")
	//initSchemas()
}

func initSchemas() {
	db.MustExec(`CREATE TABLE IF NOT EXISTS shops (
		shop_id INT PRIMARY KEY,
		name TEXT,
		last_update_time INT
	)
	`)

	db.MustExec(`CREATE TABLE IF NOT EXISTS goods (
		good_id BIGINT PRIMARY KEY,
		internal_id TEXT,
		price DECIMAL(10, 2) NOT NULL,
		shop_id INT NOT NULL,
		added_at TEXT,
		category INT NOT NULL,
		score DOUBLE PRECISION,
		sort_score DOUBLE PRECISION,
		title TEXT,
		status SMALLINT NOT NULL
	)
	`)
	db.MustExec(`CREATE INDEX IF NOT EXISTS shop_id ON goods (shop_id)`)
	db.MustExec(`CREATE INDEX IF NOT EXISTS added_at ON goods (added_at)`)
	db.MustExec(`CREATE INDEX IF NOT EXISTS category ON goods (category)`)
	db.MustExec(`CREATE INDEX IF NOT EXISTS status ON goods (status)`)

	db.MustExec(`CREATE TABLE IF NOT EXISTS urls (
		url_id SERIAL PRIMARY KEY,
		url TEXT NOT NULL,
		sha512_16k BYTEA
	)
	`)
	db.MustExec(`CREATE UNIQUE INDEX IF NOT EXISTS url ON urls (url)`)

	db.MustExec(`CREATE TABLE IF NOT EXISTS images (
		good_id BIGINT,
		url_id INT NOT NULL
	)
	`)
	db.MustExec(`CREATE UNIQUE INDEX IF NOT EXISTS good_image ON images (good_id, url_id)`)
	db.MustExec(`CREATE INDEX IF NOT EXISTS url_id ON images (url_id)`)
}
