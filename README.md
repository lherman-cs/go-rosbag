# go-rosbag

[![PkgGoDev](https://pkg.go.dev/badge/github.com/lherman-cs/go-rosbag)](https://pkg.go.dev/github.com/lherman-cs/go-rosbag)
[![CI](https://github.com/lherman-cs/go-rosbag/actions/workflows/ci.yaml/badge.svg)](https://github.com/lherman-cs/go-rosbag/actions/workflows/ci.yaml)

Go [Rosbag](http://wiki.ros.org/rosbag) parser. Designed to provide **ease of use**, **speed**, and **streamability**.

## Usage

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
			// As the API name is called, the variable "data" 
			// will be filled and used for users to access the 
			// underlying ROS messages in Go structure, it's not 
			// meant to be mutated. The underlying buffer is 
			// NOT COPIED to data, meaning that any modification to 
			// referenced objects will also modify the buffer, and
			// this is also true the other way around. Since go-rosbag
			// reuses the underlying buffer for the subsequent records 
			// (this is done by calling record.Close()), the next Read 
			// call WILL OVERWRITE the current underlying buffer. 
			// Meaning, the variable "data" MUST NOT be used after the 
			// next read. All of the data that need to be used after 
			// the subsequent reads MUST BE COPIED. Copying data can be 
			// done in a manual way, or you can also explicitly 
			// skip record.Close(). By not calling Close, it'll tell 
			// go-rosbag to not reuse the buffer, thus it "copies" 
			// the buffer.
			_ = record.ViewAs(data)
			fmt.Println(data)
		}
    
		record.Close()
	}
}
```

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
