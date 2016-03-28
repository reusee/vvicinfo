// +build no

package main

import (
	"os/exec"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/jmoiron/sqlx"
)

import "fmt"

func main() {
	date := time.Now().Format("2006_01_02")
	category := 50010850 // 连衣裙
	minGoodsNum := 10
	addedFromDate := "2016-03-01"
	maxShopId := 30000
	page := 1
	limitNum := 20
	browser := "chromium"
	titleLike := "%牛仔%"

	db, err := sqlx.Connect("mysql", "root:ffffff@tcp(127.0.0.1:3306)/vvic?parseTime=true&autocommit=true")
	ce(err, "connect db")
	defer db.Close()

	var res []struct {
		Avg_score float64
		Shop_id   int
		Name      string
	}
	err = db.Select(&res, `
		select avg(score) as avg_score, a.shop_id, name
		from `+date+`_goods a 
		left join `+date+`_shops b 
		on a.shop_id = b.shop_id
		where category= ? 
		and added_at > ? 
		and a.shop_id < ? 
		and title like ?
		group by shop_id 
		having count(*) > ? 
		order by avg_score desc, shop_id asc
		limit ?, ?`,
		category,
		addedFromDate,
		maxShopId,
		titleLike,
		minGoodsNum,
		page*limitNum,
		limitNum,
	)
	ce(err, "select")

	for _, row := range res {
		fmt.Printf("%s\n", row.Name)
		exec.Command(browser, fmt.Sprintf(
			"http://www.vvic.com/shop.html?shop_id=%d&q=&tcid=%d&sort=up_time-desc",
			row.Shop_id,
			category)).Start()
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
