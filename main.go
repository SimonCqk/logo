package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"time"
)

func lines(filename string) (int, error) {
	// readonly
	fd, err := os.Open(filename)
	if err != nil {
		return 0, err
	}
	defer fd.Close()
	// read 32k each time
	buf := make([]byte, 32*(1<<10))
	count := 0
	lineSep := []byte{'\n'}
	for {
		c, err := fd.Read(buf)
		if err != nil && err != io.EOF {
			return count, err
		}
		count += bytes.Count(buf[:c], lineSep)
		if err == io.EOF {
			break
		}
	}
	return count, nil
}

func main() {
	now := time.Now()
	_, err := lines("test.txt")
	if err != nil {
		fmt.Printf("error: %s\n", err.Error())
	}
	fmt.Println(time.Since(now).Nanoseconds())
}
