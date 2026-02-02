package main

import (
	"fmt"
	"time"
)

func max(data []int64) int64 {
	if len(data) == 0 {
		return 0
	}
	max := data[0]
	len := len(data)
	for i := 0; i < len; i++ {
		if data[i] > max {
			max = data[i]
		}
	}
	return max
}

func makeArray(maxValue int64) []int64 {
	result := make([]int64, maxValue)
	for i := int64(1); i <= maxValue; i++ {
		result[i-1] = i
	}
	return result
}

func main() {
	a := makeArray(99999)
	start := time.Now()
	fmt.Println(max(a))
	fmt.Println(time.Since(start))
}
