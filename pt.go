package main

import (
	"fmt"
	"time"
)

var (
	start = time.Now()
)

var outputStrs = make(chan string)

func init() {
	go func() {
		for s := range outputStrs {
			print(s)
		}
	}()
}

func pt(format string, args ...interface{}) {
	outputStrs <- fmt.Sprintf("%-20v", time.Now().Sub(start)) +
		fmt.Sprintf(format, args...)
}
