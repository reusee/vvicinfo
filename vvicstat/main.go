package main

import (
	"os"
	"os/exec"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
)

import "fmt"

func main() {
	minimalGoodsCount := 5
	earlistShelfDate := "2016-03-01"
	maxResults := 30
	browser := "chromium"
	keyword := os.Args[1]

	var err error
	db, err := sqlx.Connect("postgres", "user=reus dbname=vvic sslmode=disable")
	ce(err, "connect to db")
	defer db.Close()

	var res []struct {
		Avg_score float64
		Shop_id   int
		Count     int
	}
	err = db.Select(&res, `
		SELECT AVG(score) AS avg_score, a.shop_id, COUNT(*) as count
		FROM goods a
		LEFT JOIN shops b
		ON a.shop_id = b.shop_id

		WHERE
		added_at > $1
		AND title LIKE $2
		AND a.status > 0

		GROUP BY a.shop_id
		HAVING COUNT(*) > $3

		ORDER BY avg_score DESC, a.shop_id ASC
		--ORDER BY count DESC, a.shop_id ASC
		LIMIT $4
	`,
		earlistShelfDate,
		"%"+keyword+"%",
		minimalGoodsCount,
		maxResults,
	)
	ce(err, "select")

	for _, row := range res {
		fmt.Printf("%d\n", row.Shop_id)
		exec.Command(browser, fmt.Sprintf(
			"http://www.vvic.com/shop.html?shop_id=%d&q=%s&sort=up_time-desc",
			row.Shop_id, keyword)).Start()
	}
}
