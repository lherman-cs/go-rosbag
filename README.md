# go-rosbag
[Rosbag](http://wiki.ros.org/rosbag) parser written in Go. It's designed to be **lazy, low memory usage, fast, streamable, and easy-to-use**.

## Why?
I want to process huge Rosbag data in the backend without bringing ROS ecosystem. So, I think Go is a great language for this. Unfortunately, I couldn't find a good library to fullfil what I need.

These are the libraries that I've found in my search:

* https://github.com/starship-technologies/gobag: this is probably the closest library, but unfortunately it doesn't have a good way to process big rosbags and there's no easy way to Unmarshall the messages to structs
* https://github.com/brychanrobot/goros/blob/master/rosbag.go: this project seems to be incomplete and abandoned.


## Code Coverage

Currently, the unit tests only cover the happy path, and it's not extensive as I want to be. Fuzzing has been added for testing ROS message decoding to cover data corruption. I'm still working on adding coverage during my free time.
