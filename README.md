# go-rosbag
Go [Rosbag](http://wiki.ros.org/rosbag) parser. Designed to provide **ease of use, low memory usage, speed, streamability, and lazy operations**.

## Why?
Processing Rosbags in Go, especially huge Rosbags was non-trivial, go-rosbag aims to add unofficial support for Rosbag parsing in Go.

There is offical Rosbag parsing support in Python and C++. There is unofficial Rosbag parsing support in Javascript. But I, and imagine you do as well, just love writing in Go 😍. 

## Code Coverage

Currently, the unit tests only cover the happy path, and it's not extensive as I want to be. Fuzzing has been added for testing ROS message decoding to cover data corruption. I'm still working on adding coverage during my free time.

## Alternatives

There are two known alternatives to go-rosbag: 

* https://github.com/starship-technologies/gobag: This project seems to be lacking a good way to process large rosbags. Additionally seems to lack an easy way to Unmarshall messages to structs.
* https://github.com/brychanrobot/goros/blob/master/rosbag.go: This project seems to be incomplete and abandoned.

## License

[Apache 2.0](https://github.com/lherman-cs/go-rosbag/blob/master/LICENSE)
