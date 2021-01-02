package rosbag

import (
	"io"
	"os"
	"testing"
)

func must(err error) {
	if err != nil {
		panic(err)
	}
}

func BenchmarkE2E(b *testing.B) {
	for i := 0; i < b.N; i++ {
		start()
	}
}

func start() {
	f, err := os.Open("examples/logging/example.bag")
	must(err)
	defer f.Close()

	decoder := NewDecoder(f)

	for {
		record, err := decoder.Read()
		if err != nil {
			if err == io.EOF {
				break
			}
			must(err)
		}

		switch record := record.(type) {
		case *RecordMessageData:
			_, err := record.Transform()
			must(err)
		}
		record.Close()
	}
}
