package main

import (
	"fmt"
	"os"

	"github.com/lherman-cs/go-rosbag"
)

func must(err error) {
	if err != nil {
		panic(err)
	}
}

func main() {
	f, err := os.Open("example.bag")
	must(err)
	defer f.Close()

	decoder := rosbag.NewDecoder(f)

	for {
		record, err := decoder.Next()
		must(err)

		fmt.Println(record)
	}
}
