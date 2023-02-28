package plugin

import "google.golang.org/protobuf/compiler/protogen"

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
