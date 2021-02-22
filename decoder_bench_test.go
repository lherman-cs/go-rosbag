package rosbag

import (
	"io"
	"net/http"
	"os"
	"testing"
	"time"
)

func BenchmarkE2E(b *testing.B) {
	const bagName = "benchbag.bag"
	b.StopTimer()

	var bag *os.File
	if _, err := os.Stat(bagName); err != nil {
		resp, err := http.Get("https://open-source-webviz-ui.s3.amazonaws.com/demo.bag")
		if err != nil {
			b.Fatal(err)
		}
		defer resp.Body.Close()

		bag, err = os.Create(bagName)
		if err != nil {
			b.Fatal(err)
		}

		_, err = io.Copy(bag, resp.Body)
		if err != nil {
			bag.Close()
			b.Fatal(err)
		}
	} else {
		bag, err = os.Open(bagName)
		if err != nil {
			b.Fatal(err)
		}
	}
	defer bag.Close()

	bagStat, err := bag.Stat()
	if err != nil {
		b.Fatal(err)
	}

	bagSizeInMB := float64(bagStat.Size()) / 1024 / 1024
	b.Logf("Benchmarking with %f MB rosbag", bagSizeInMB)
	count := 0
	startedAt := time.Now()

	b.StartTimer()
	for i := 0; i < b.N; i++ {
		start(b, bag.Name())
		count++
	}
	b.StopTimer()

	elapsed := time.Now().Sub(startedAt)
	b.Logf(`
==== Average Statistics ====

Time taken: %d ms
Throughput: %.2f GB/s
	`, elapsed.Milliseconds()/int64(count), float64(count)*bagSizeInMB/1024/elapsed.Seconds())
}

func start(b *testing.B, bag string) {
	f, err := os.Open(bag)
	if err != nil {
		b.Fatal(err)
	}
	defer f.Close()

	decoder := NewDecoder(f)
	for {
		record, err := decoder.Read()
		if err != nil {
			if err == io.EOF {
				break
			}
			b.Fatal(err)
		}

		switch record := record.(type) {
		case *RecordMessageData:
			data := make(map[string]interface{})
			err := record.ViewAs(data)
			if err != nil {
				b.Fatal(err)
			}
		}
		record.Close()
	}
}
