package main

import (
	"os"

	"github.com/k0kubun/pp"
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
		record, err := decoder.Read()
		must(err)

		switch record := record.(type) {
		case *rosbag.RecordMessageData:
			data := make(map[string]interface{})
			err = record.ViewAs(data)
			must(err)
			pp.Println(data)
		}

		record.Close()
	}
}
