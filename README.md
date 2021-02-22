# go-rosbag
Go [Rosbag](http://wiki.ros.org/rosbag) parser. Designed to provide **ease of use, low memory usage, speed, streamability, and lazy operations**.

## Why?
Processing Rosbags in Go, especially huge Rosbags was non-trivial, go-rosbag aims to add unofficial support for Rosbag parsing in Go.

There is offical Rosbag parsing support in Python and C++. There is unofficial Rosbag parsing support in Javascript. But I, and imagine you do as well, just love writing in Go 😍. 

## Benchmark

Benchmark source code: .

Hardware specs:

Model: MacBook Pro (15-inch, 2017)
Processor: 2.8 GHz Quad-Core Intel Core i7
Memory: 16 GB 2133 MHz LPDDR3
Storage: 256GB PCIe-based onboard SSD

The following benchmark ([source](https://github.com/lherman-cs/go-rosbag/blob/bb8c5d16d3b51ca42f137c8214b07446eaea25a0/decoder_bench_test.go)) was taken by parsing a 696 MB rosbag file from [webviz](https://webviz.io/). The bag contains frames and metadata from a car.

```
Time taken: 147 ms
Throughput: 4.59 GB/s
Memory usage: 33.60 MB
```

For a reference, `time cat <bag> > /dev/null` takes 118 ms. This means, `go-rosbag` only adds **29 ms** to the total runtime!

## Examples

Please see the [examples](examples) directory within this repository.


## Code Coverage

Currently, the unit tests only cover the happy path, and it's not extensive as I want to be. Fuzzing has been added for testing ROS message decoding to cover data corruption. I'm still working on adding coverage during my free time.

## Alternatives

There are two known alternatives to go-rosbag: 

* https://github.com/starship-technologies/gobag: This project seems to be lacking a good way to process large rosbags. Additionally seems to lack an easy way to Unmarshall messages to structs.
* https://github.com/brychanrobot/goros/blob/master/rosbag.go: This project seems to be incomplete and abandoned.

## License

[Apache 2.0](https://github.com/lherman-cs/go-rosbag/blob/master/LICENSE)
