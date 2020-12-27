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

		// fmt.Println(record)
		chunkRecord, ok := record.(*rosbag.RecordChunk)
		if !ok {
			continue
		}

		must(handleChunkRecord(chunkRecord))
	}
}

func handleChunkRecord(chunkRecord *rosbag.RecordChunk) error {
	for {
		record, err := chunkRecord.Next()
		if err != nil {
			if err == io.EOF {
				return nil
			} else {
				must(err)
			}
		}

		switch record := record.(type) {
		case *rosbag.RecordConnection:
			fmt.Println(record)
			hdr, err := record.ConnectionHeader()
			must(err)
			fmt.Println(hdr)
		}

		if err != nil {
			return err
		}
	}
}

func handleMessage(message *rosbag.RecordMessageData) error {
	fmt.Println(message.Time)
	return nil
}
