test:
	ENV=test go test . -cover

lint:
	golangci-lint run