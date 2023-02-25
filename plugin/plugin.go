package plugin

import (
	"strconv"

	"google.golang.org/protobuf/compiler/protogen"
)

var imports = []string{
	"context",
	"github.com/autom8ter/protoc-gen-rabbitmq/rpc",
	"github.com/golang/protobuf/proto",
}

func generateServer(genFile *protogen.GeneratedFile, s *protogen.Service) {
	var serverName = s.GoName + "Server"
	//var clientName = s.GoName + "Client"
	genFile.P("// ", serverName, " is a RabbitMQ service")
	genFile.P("type ", serverName, " interface {")
	for _, m := range s.Methods {
		genFile.P(m.GoName, "(ctx context.Context, in *", m.Input.GoIdent, ") (*", m.Output.GoIdent, ", error)")
	}
	genFile.P("}")
	genFile.P(`func Serve(ctx context.Context, srv *rpc.Server, handler `, serverName, `) error {`)
	genFile.P(`return srv.Serve(ctx, func(ctx context.Context, msg rpc.Message) rpc.Message {`)
	genFile.P(`meta := msg.Metadata`)
	genFile.P(`switch msg.Method {`)
	for _, m := range s.Methods {
		genFile.P(`case "` + m.GoName + `":`)
		genFile.P(`var in `, m.Input.GoIdent)
		genFile.P(`if err := proto.Unmarshal(msg.Body, &in); err != nil {`)
		genFile.P(`return rpc.Message{`)
		genFile.P(`ID:       msg.ID,`)
		genFile.P(`Method:   msg.Method,`)
		genFile.P(`Metadata: meta,`)
		genFile.P(`Error:    err,`)
		genFile.P(`}`)
		genFile.P(`}`)
		genFile.P(`out, err := handler.`, m.GoName, `(ctx, &in)`)
		genFile.P(`if err != nil {`)
		genFile.P(`return rpc.Message{`)
		genFile.P(`ID:       msg.ID,`)
		genFile.P(`Method:   msg.Method,`)
		genFile.P(`Metadata: meta,`)
		genFile.P(`Error:    err,`)
		genFile.P(`}`)
		genFile.P(`}`)
		genFile.P(`body, err := proto.Marshal(out)`)
		genFile.P(`if err != nil {`)
		genFile.P(`return rpc.Message{`)
		genFile.P(`ID:       msg.ID,`)
		genFile.P(`Method:   msg.Method,`)
		genFile.P(`Metadata: meta,`)
		genFile.P(`Error:    err,`)
		genFile.P(`}`)
		genFile.P(`}`)
		genFile.P(`return rpc.Message{`)
		genFile.P(`ID:       msg.ID,`)
		genFile.P(`Method:   msg.Method,`)
		genFile.P(`Metadata: meta,`)
		genFile.P(`Body:     body,`)
		genFile.P(`}`)
	}
	genFile.P(`}`)
	genFile.P(`return rpc.Message{`)
	genFile.P(`ID:       msg.ID,`)
	genFile.P(`Method:   msg.Method,`)
	genFile.P(`Metadata: meta,`)
	genFile.P(`Error:    rpc.ErrUnsupportedMethod,`)
	genFile.P(`}`)
	genFile.P(`})`)
	genFile.P(`}`)
}

func generateClient(genFile *protogen.GeneratedFile, s *protogen.Service) {
	var clientName = s.GoName + "Client"
	genFile.P("// ", clientName, " is a RabbitMQ client")
	genFile.P("type ", clientName, " struct {")
	genFile.P("client *rpc.Client")
	genFile.P("}")
	genFile.P("func New", clientName, "(client *rpc.Client) *", clientName, " {")
	genFile.P("return &", clientName, "{client: client}")
	genFile.P("}")
	for _, m := range s.Methods {
		genFile.P("func (c *", clientName, ") ", m.GoName, "(ctx context.Context, in *", m.Input.GoIdent, ") (*", m.Output.GoIdent, ", error) {")
		genFile.P("var out ", m.Output.GoIdent)
		genFile.P("body, err := proto.Marshal(in)")
		genFile.P("if err != nil {")
		genFile.P("return nil, err")
		genFile.P("}")
		genFile.P("msg, err := c.client.Request(ctx, rpc.Message{Method: \"", m.GoName, "\", Body: body})")
		genFile.P("if err != nil {")
		genFile.P("return nil, err")
		genFile.P("}")
		genFile.P("if msg.Error != nil {")
		genFile.P("return nil, msg.Error")
		genFile.P("}")
		genFile.P("if err := proto.Unmarshal(msg.Body, &out); err != nil {")
		genFile.P("return nil, err")
		genFile.P("}")
		genFile.P("return &out, nil")
		genFile.P("}")
	}
}

func Plugin() func(gen *protogen.Plugin) error {
	return func(gen *protogen.Plugin) error {
		for _, f := range gen.Files {
			if !f.Generate {
				continue
			}
			genFile := gen.NewGeneratedFile(f.GeneratedFilenamePrefix+".rabbitmq.go", f.GoImportPath)
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
