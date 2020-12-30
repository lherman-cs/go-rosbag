package main

import (
	"fmt"
	"os"
	"runtime/pprof"

	"github.com/k0kubun/pp"
	"github.com/lherman-cs/go-rosbag"
)

func must(err error) {
	if err != nil {
		panic(err)
	}
}

func main() {
	cpu, err := os.Create("cpu.out")
	must(err)
	defer cpu.Close()

	must(pprof.StartCPUProfile(cpu))
	defer pprof.StopCPUProfile()
	f, err := os.Open("example.bag")
	must(err)
	defer f.Close()

	decoder := rosbag.NewDecoder(f)
	var record rosbag.Record

	visited := make(map[uint32]struct{})

	for {
		op, err := decoder.Read(&record)
		must(err)

		if op == rosbag.OpMessageData {
			connId, err := record.Conn()
			must(err)

			if _, ok := visited[connId]; ok {
				continue
			}

			conn := record.Conns[connId]
			fmt.Println(conn.Topic)

			v := make(map[string]interface{})
			must(record.UnmarshallTo(v))
			pp.Println(v)

			visited[connId] = struct{}{}
		}
	}
}
