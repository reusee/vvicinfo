package main

import (
	"crypto/sha512"
	"io"
	"os"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
)

import "fmt"

var db *sqlx.DB

func init() {
	var err error
	db, err = sqlx.Connect("postgres", "user=reus dbname=vvic sslmode=disable")
	ce(err, "connect to db")
}

func main() {
	ids := make(map[int]bool)
	for _, filePath := range os.Args[1:] {
		f, err := os.Open(filePath)
		ce(err, "open file")
		defer f.Close()

		h := sha512.New()
		_, err = io.CopyN(h, f, 16384)
		ce(err, "hash")

		sum := h.Sum(nil)

		var goodIds []int
		err = db.Select(&goodIds, `
			SELECT g.good_id FROM goods g
			LEFT JOIN images i
			ON g.good_id = i.good_id
			LEFT JOIN urls u
			ON i.url_id = u.url_id

			WHERE sha512_16k = $1
			ORDER BY g.status DESC
			`,
			sum)
		ce(err, "find good ids")

		for _, id := range goodIds {
			ids[id] = true
		}
	}
	for id := range ids {
		fmt.Printf("http://www.vvic.com/item.html?id=%d\n", id)
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

func (e *Err) Origin() error {
	var ret error = e
	for err, ok := ret.(*Err); ok && err.Prev != nil; err, ok = ret.(*Err) {
		ret = err.Prev
	}
	return ret
}

func me(err error, format string, args ...interface{}) *Err {
	if len(args) > 0 {
		return &Err{
			Pkg:  `vvicinfofind`,
			Info: fmt.Sprintf(format, args...),
			Prev: err,
		}
	}
	return &Err{
		Pkg:  `vvicinfofind`,
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
