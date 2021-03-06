package main

import (
	"math/rand"
	"sort"
)

type Strs []string

func (s Strs) Reduce(initial interface{}, fn func(value interface{}, elem string) interface{}) (ret interface{}) {
	ret = initial
	for _, elem := range s {
		ret = fn(ret, elem)
	}
	return
}

func (s Strs) Map(fn func(string) string) (ret Strs) {
	for _, elem := range s {
		ret = append(ret, fn(elem))
	}
	return
}

func (s Strs) Filter(filter func(string) bool) (ret Strs) {
	for _, elem := range s {
		if filter(elem) {
			ret = append(ret, elem)
		}
	}
	return
}

func (s Strs) All(predict func(string) bool) (ret bool) {
	ret = true
	for _, elem := range s {
		ret = predict(elem) && ret
	}
	return
}

func (s Strs) Any(predict func(string) bool) (ret bool) {
	for _, elem := range s {
		ret = predict(elem) || ret
	}
	return
}

func (s Strs) Each(fn func(e string)) {
	for _, elem := range s {
		fn(elem)
	}
}

func (s Strs) Shuffle() {
	for i := len(s) - 1; i >= 1; i-- {
		j := rand.Intn(i + 1)
		s[i], s[j] = s[j], s[i]
	}
}

func (s Strs) Sort(cmp func(a, b string) bool) {
	sorter := sliceSorter{
		l: len(s),
		less: func(i, j int) bool {
			return cmp(s[i], s[j])
		},
		swap: func(i, j int) {
			s[i], s[j] = s[j], s[i]
		},
	}
	_ = sorter.Len
	_ = sorter.Less
	_ = sorter.Swap
	sort.Sort(sorter)
}

type sliceSorter struct {
	l    int
	less func(i, j int) bool
	swap func(i, j int)
}

func (t sliceSorter) Len() int {
	return t.l
}

func (t sliceSorter) Less(i, j int) bool {
	return t.less(i, j)
}

func (t sliceSorter) Swap(i, j int) {
	t.swap(i, j)
}

func (s Strs) Clone() (ret Strs) {
	ret = make([]string, len(s))
	copy(ret, s)
	return
}
