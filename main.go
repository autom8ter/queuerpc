package main

import (
	"flag"

	"github.com/autom8ter/protoc-gen-rabbitmq/plugin"
	"google.golang.org/protobuf/compiler/protogen"
)

func main() {
	var flags flag.FlagSet
	flag.Parse()
	protogen.Options{
		ParamFunc: flags.Set,
	}.Run(plugin.Plugin())
}
