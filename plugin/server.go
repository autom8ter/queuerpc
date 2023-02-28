package plugin

import "google.golang.org/protobuf/compiler/protogen"

func generateUnaryServerHandler(genFile *protogen.GeneratedFile, s *protogen.Service) {
	genFile.P(`UnaryHandler:        func(ctx context.Context, msg *queuerpc.Message) *queuerpc.Message {`)
	genFile.P(`meta := msg.Metadata`)
	genFile.P(`switch msg.Method {`)
	for _, m := range s.Methods {
		if m.Desc.IsStreamingServer() || m.Desc.IsStreamingClient() {
			continue
		}
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
}

func generateServerStreamHandler(genFile *protogen.GeneratedFile, s *protogen.Service) {
	genFile.P(`ServerStreamHandler: func(ctx context.Context, msg *queuerpc.Message) (<-chan *queuerpc.Message, error) {`)
	genFile.P(`meta := msg.Metadata`)
	genFile.P(`ch := make(chan *queuerpc.Message)`)
	genFile.P(`switch msg.Method {`)
	for _, m := range s.Methods {
		if !m.Desc.IsStreamingServer() || m.Desc.IsStreamingClient() {
			continue
		}
		genFile.P(`default:`)
		genFile.P(`return nil, queuerpc.ErrUnsupportedMethod`)
		genFile.P(`case "` + m.GoName + `":`)
		genFile.P(`var in `, m.Input.GoIdent)
		genFile.P(`if err := proto.Unmarshal(msg.Body, &in); err != nil {`)
		genFile.P(`return nil, queuerpc.ErrUnmarshal`)
		genFile.P(`}`)
		genFile.P(`out, err := handler.`, m.GoName, `(queuerpc.NewContextWithMetadata(ctx, meta), &in)`)
		genFile.P(`if err != nil {`)
		genFile.P(`return nil, err`)
		genFile.P(`}`)
		genFile.P(`go func() {`)
		genFile.P(`defer close(ch)`)
		genFile.P(`ctx, cancel := context.WithCancel(ctx)`)
		genFile.P(`defer cancel()`)
		genFile.P(`for {`)
		genFile.P(`select {`)
		genFile.P(`case <-ctx.Done():`)
		genFile.P(`return`)
		genFile.P(`case out, ok := <-out:`)
		genFile.P(`if !ok {`)
		genFile.P(`return`)
		genFile.P(`}`)
		genFile.P(`body, _ := proto.Marshal(out)`)
		genFile.P(`ch <- &queuerpc.Message{`)
		genFile.P(`Id:       msg.Id,`)
		genFile.P(`Method:   msg.Method,`)
		genFile.P(`Metadata: meta,`)
		genFile.P(`Body:     body,`)
		genFile.P(`}`)
		genFile.P(`}`)
		genFile.P(`}`)

		genFile.P(`}()`)
		genFile.P(`return ch, nil`)
	}
	genFile.P(`}`)
	genFile.P(`},`)
}

func generateServer(genFile *protogen.GeneratedFile, s *protogen.Service) {
	var serverName = s.GoName + "Server"
	//var clientName = s.GoName + "Client"
	genFile.P("// ", serverName, " is a type safe RabbitMQ rpc server")
	genFile.P("type ", serverName, " interface {")
	for _, m := range s.Methods {
		switch {
		case !m.Desc.IsStreamingServer() && !m.Desc.IsStreamingClient():
			genFile.P(m.Comments.Leading.String(), m.GoName, "(ctx context.Context, in *", m.Input.GoIdent, ") (*", m.Output.GoIdent, ", error)")
		case m.Desc.IsStreamingServer() && !m.Desc.IsStreamingClient():
			genFile.P(m.Comments.Leading.String(), m.GoName, "(ctx context.Context, in *", m.Input.GoIdent, ") (<-chan *", m.Output.GoIdent, ", error)")
		default:
			panic("client streaming / bidi streaming is not supported")
		}
	}
	genFile.P("}")
	genFile.P("// Serve starts the server and blocks until the context is canceled or the deadline is exceeded")
	genFile.P(`func Serve(srv queuerpc.IServer, handler `, serverName, `) error {`)
	genFile.P(`return srv.Serve(queuerpc.Handlers{`)
	generateUnaryServerHandler(genFile, s)
	generateServerStreamHandler(genFile, s)
	genFile.P(`})`)
	genFile.P(`}`)
}
