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

func generateServer(genFile *protogen.GeneratedFile, s *protogen.Service) {
	var serverName = s.GoName + "Server"
	//var clientName = s.GoName + "Client"
	genFile.P("// ", serverName, " is a type safe RabbitMQ rpc server")
	genFile.P("type ", serverName, " interface {")
	for _, m := range s.Methods {
		genFile.P(m.Comments.Leading.String(), m.GoName, "(ctx context.Context, in *", m.Input.GoIdent, ") (*", m.Output.GoIdent, ", error)")
	}
	genFile.P("}")
	genFile.P("// Serve starts the server and blocks until the context is canceled or the deadline is exceeded")
	genFile.P(`func Serve(srv queuerpc.IServer, handler `, serverName, `) error {`)
	genFile.P(`return srv.Serve(queuerpc.Handlers{`)
	genFile.P(`UnaryHandler:        func(ctx context.Context, msg *queuerpc.Message) *queuerpc.Message {`)
	genFile.P(`meta := msg.Metadata`)
	genFile.P(`switch msg.Method {`)
	for _, m := range s.Methods {
		genFile.P(`case "` + m.GoName + `":`)
		genFile.P(`var in `, m.Input.GoIdent)
		genFile.P(`if err := proto.Unmarshal(msg.Body, &in); err != nil {`)
		genFile.P(`return &queuerpc.Message{`)
		genFile.P(`Id:       msg.Id,`)
		genFile.P(`Method:   msg.Method,`)
		genFile.P(`Metadata: meta,`)
		genFile.P(`Error:    queuerpc.ErrUnmarshal,`)
		genFile.P(`}`)
		genFile.P(`}`)
		genFile.P(`out, err := handler.`, m.GoName, `(queuerpc.NewContextWithMetadata(ctx, meta), &in)`)
		genFile.P(`if err != nil {`)
		genFile.P(`return &queuerpc.Message{`)
		genFile.P(`Id:       msg.Id,`)
		genFile.P(`Method:   msg.Method,`)
		genFile.P(`Metadata: meta,`)
		genFile.P(`Error:    queuerpc.ErrorFrom(err),`)
		genFile.P(`}`)
		genFile.P(`}`)
		genFile.P(`body, err := proto.Marshal(out)`)
		genFile.P(`if err != nil {`)
		genFile.P(`return &queuerpc.Message{`)
		genFile.P(`Id:       msg.Id,`)
		genFile.P(`Method:   msg.Method,`)
		genFile.P(`Metadata: meta,`)
		genFile.P(`Error:    queuerpc.ErrMarshal,`)
		genFile.P(`}`)
		genFile.P(`}`)
		genFile.P(`return &queuerpc.Message{`)
		genFile.P(`Id:       msg.Id,`)
		genFile.P(`Method:   msg.Method,`)
		genFile.P(`Metadata: meta,`)
		genFile.P(`Body:     body,`)
		genFile.P(`}`)
	}
	genFile.P(`}`)
	genFile.P(`return &queuerpc.Message{`)
	genFile.P(`Id:       msg.Id,`)
	genFile.P(`Method:   msg.Method,`)
	genFile.P(`Metadata: meta,`)
	genFile.P(`Error:    queuerpc.ErrUnsupportedMethod,`)
	genFile.P(`}`)
	genFile.P(`},`)
	genFile.P(`ClientStreamHandler: nil,`)
	genFile.P(`ServerStreamHandler: nil,`)
	genFile.P(`})`)
}

func generateClient(genFile *protogen.GeneratedFile, s *protogen.Service) {
	var clientName = s.GoName + "Client"
	genFile.P("// ", clientName, " is a type safe RabbitMQ rpc client")
	genFile.P("type ", clientName, " struct {")
	genFile.P("client queuerpc.IClient")
	genFile.P("}")
	genFile.P("// New", clientName, " returns a new ", clientName, "with the given rpc client")
	genFile.P("func New", clientName, "(client queuerpc.IClient) *", clientName, " {")
	genFile.P("return &", clientName, "{client: client}")
	genFile.P("}")
	genFile.P("\n")
	for _, m := range s.Methods {
		genFile.P(m.Comments.Leading.String(), "func (c *", clientName, ") ", m.GoName, "(ctx context.Context, in *", m.Input.GoIdent, ") (*", m.Output.GoIdent, ", error) {")
		genFile.P("meta := queuerpc.MetadataFromContext(ctx)")
		genFile.P("var out ", m.Output.GoIdent)
		genFile.P("body, err := proto.Marshal(in)")
		genFile.P("if err != nil {")
		genFile.P("return nil, err")
		genFile.P("}")
		genFile.P("msg, err := c.client.Request(ctx, &queuerpc.Message{Method: \"", m.GoName, "\", Body: body, Metadata: meta})")
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
