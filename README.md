# go-rosbag

[![PkgGoDev](https://pkg.go.dev/badge/github.com/lherman-cs/go-rosbag)](https://pkg.go.dev/github.com/lherman-cs/go-rosbag)
[![CI](https://github.com/lherman-cs/go-rosbag/actions/workflows/ci.yaml/badge.svg)](https://github.com/lherman-cs/go-rosbag/actions/workflows/ci.yaml)

Go [Rosbag](http://wiki.ros.org/rosbag) parser. Designed to provide **ease of use**, **speed**, and **streamability**.

## Usage

### View Messages as map[string]interface{}

```go
package main

import (
	"fmt"
	"io"
	"os"

	"github.com/lherman-cs/go-rosbag"
)

func main() {
	f, _ := os.Open("example.bag")
	defer f.Close()

	decoder := rosbag.NewDecoder(f)
	for {
		record, err := decoder.Read()
		if err == io.EOF {
			break
		}

		switch record := record.(type) {
		case *rosbag.RecordMessageData:
			data := make(map[string]interface{})
			// After ViewAs gets called, the variable "data" will be used to map
			// the underlying buffer to Go data structures, and they'll SHARE the
			// same underlying buffer.
			_ = record.ViewAs(data)
			fmt.Println(data)
		}
    
    		// Mark the current record is no longer used, the underlying buffer can be reused.
		record.Close()
	}
}
```

### View Messages as structs

```go
package main

import (
	"fmt"
	"io"
	"os"

	"github.com/lherman-cs/go-rosbag"
)

type Pixel struct {
	R uint8	`rosbag:"r"`
	G uint8	`rosbag:"g"`
	B uint8	`rosbag:"b"`
}

func main() {
	f, _ := os.Open("example.bag")
	defer f.Close()

	decoder := rosbag.NewDecoder(f)
	for {
		record, err := decoder.Read()
		if err == io.EOF {
			break
		}

		switch record := record.(type) {
		case *rosbag.RecordMessageData:
			if record.ConnectionHeader().Topic != "/pixel" {
				continue
			}
			
			var pixel Pixel
			_ = record.ViewAs(&pixel)
			fmt.Println(pixel)
		}
    
    		// Mark the current record is no longer used, the underlying buffer can be reused.
		record.Close()
	}
}
```

## Data Type Mapping

### Primitive Types

|ROS Type|Go Type|
|:--:|:--:|
|bool|bool|
|int8|int8|
|byte|int8|
|uint8|uint8|
|char|uint8|
|int16|int16|
|uint16|uint16|
|int32|uint32|
|int64|uint64|
|float32|float32|
|float64|float64|
|string|string|
|time|[time.Time](https://golang.org/pkg/time/#Time)|
|duration|[time.Duration](https://golang.org/pkg/time/#Duration)|

### Array Handling

Both fixed-length and variable-length are mapped to Go slices. For example, uint8[] with a length of 3 and uint8[3] will be mapped to []uint8 in Go.

## Benchmark

Hardware specs:

* Model: MacBook Pro (15-inch, 2017)
* Processor: 2.8 GHz Quad-Core Intel Core i7
* Memory: 16 GB 2133 MHz LPDDR3
* Storage: 256GB PCIe-based onboard SSD

The following benchmark result ([source](https://github.com/lherman-cs/go-rosbag/blob/bb8c5d16d3b51ca42f137c8214b07446eaea25a0/decoder_bench_test.go)) was taken by parsing a 696 MB rosbag file from [webviz](https://webviz.io/). The bag contains frames and metadata from a car.

```
Time taken: 147 ms
Throughput: 4.59 GB/s
Memory usage: 33.60 MB
```

For a reference, `time cat <bag> > /dev/null` takes 118 ms. This means, `go-rosbag` only adds **29 ms** to the total runtime!

## Examples

Please see the [examples](examples) directory within this repository.


## Code Coverage

Currently, the unit tests only cover the happy path, and it's not extensive as I want it to be. I'm still working on adding coverage during my free time.

## Alternatives

There are two known alternatives to go-rosbag (at least that I know of): 

* https://github.com/starship-technologies/gobag: This project seems to be lacking a good way to process large rosbags, and unmarshall messages to structs.
* https://github.com/brychanrobot/goros/blob/master/rosbag.go: This project seems to be incomplete and abandoned.

## Contributing

Any contribution is welcomed! I will review pull requests as soon as possible.

## License

go-rosbag is licensed under Apache license version 2.0. See LICENSE file.
