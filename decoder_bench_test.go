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
	var record Record

	for {
		op, err := decoder.Read(&record)
		if err != nil {
			if err == io.EOF {
				break
			}
			must(err)
		}

		if op == OpMessageData {
			v := make(map[string]interface{})
			must(record.UnmarshallTo(v))
		}
	}
}
