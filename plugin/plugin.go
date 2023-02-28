package plugin

import (
	"strconv"

	"google.golang.org/protobuf/compiler/protogen"
)

var imports = []string{
	"context",
	"github.com/autom8ter/queuerpc",
	"github.com/golang/protobuf/proto",
}

// Plugin is a protoc plugin that generates RabbitMQ rpc bindings for Go.
func Plugin() func(gen *protogen.Plugin) error {
	return func(gen *protogen.Plugin) error {
		for _, f := range gen.Files {
			if !f.Generate {
				continue
			}
			genFile := gen.NewGeneratedFile(f.GeneratedFilenamePrefix+".queuerpc.go", f.GoImportPath)
			// add package
			genFile.P("package ", f.GoPackageName)
			// add imports
			for _, imp := range imports {
				genFile.P("import ", strconv.Quote(imp))
			}
			// add service
			for _, s := range f.Services {
				generateServer(genFile, s)
				generateClient(genFile, s)
			}
		}
		return nil
	}
}
