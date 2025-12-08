test:
	ENV=test go test . -cover

lint:
	scopeguard ./... && golangci-lint run