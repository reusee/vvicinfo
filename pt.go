package main

import (
	"fmt"
	"time"
)

var (
	start = time.Now()
)

var outputStrs = make(chan string, 1024)

func init() {
	go func() {
		for s := range outputStrs {
			print(s)
		}
	}()
}

func pt(format string, args ...interface{}) {
	now := time.Now()
	outputStrs <- fmt.Sprintf("%s %-20v", now.Format("15:04:05"), now.Sub(start)) +
		fmt.Sprintf(format, args...)
}
