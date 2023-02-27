install:
	@cd protoc-gen-queuerpc; go install
gen-example: install
	@cd example; buf generate

gen:
	@buf generate