package main

import (
	"fmt"
	"io"
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
		chunkRecord, ok := record.(*rosbag.RecordChunk)
		if !ok {
			continue
		}

		for {
			record, err = chunkRecord.Next()
			if err != nil {
				if err == io.EOF {
					break
				} else {
					must(err)
				}
			}

			fmt.Println(record)
		}
	}
}
