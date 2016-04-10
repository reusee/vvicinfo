// +build no

package main

import (
	"os"
	"os/exec"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
)

import "fmt"

var categories = map[string]int{
	"连衣裙": 50010850,
}

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
	}
	err = db.Select(&res, `
		SELECT AVG(score) AS avg_score, a.shop_id
		FROM goods a
		LEFT JOIN shops b
		ON a.shop_id = b.shop_id

		WHERE
		added_at > $1
		AND title LIKE $2

		GROUP BY a.shop_id
		HAVING COUNT(*) > $3

		ORDER BY avg_score DESC, a.shop_id ASC
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

type Err struct {
	Pkg  string
	Info string
	Prev error
}

func (e *Err) Error() string {
	if e.Prev == nil {
		return fmt.Sprintf("%s: %s", e.Pkg, e.Info)
	}
	return fmt.Sprintf("%s: %s\n%v", e.Pkg, e.Info, e.Prev)
}

func me(err error, format string, args ...interface{}) *Err {
	if len(args) > 0 {
		return &Err{
			Pkg:  `vvicinfoopen`,
			Info: fmt.Sprintf(format, args...),
			Prev: err,
		}
	}
	return &Err{
		Pkg:  `vvicinfoopen`,
		Info: format,
		Prev: err,
	}
}

func ce(err error, format string, args ...interface{}) {
	if err != nil {
		panic(me(err, format, args...))
	}
}

func ct(err *error) {
	if p := recover(); p != nil {
		if e, ok := p.(error); ok {
			*err = e
		} else {
			panic(p)
		}
	}
}
