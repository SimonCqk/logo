package main

import "fmt"

func main() {
	l := []int{1, 2, 3, 4, 5, 6, 7}
	idx := 0
	for _, num := range l {
		if num == 5 {
			break
		}
		idx++
	}
	copy(l[idx:], l[idx+1:])
	l = l[:len(l)-1]
	fmt.Printf("%v", l)
}
