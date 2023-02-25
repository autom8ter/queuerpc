install:
	@go install
gen-example: install
	@cd example; buf generate
