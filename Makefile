test:
	ENV=test go test . -cover -coverprofile=c.out

coverage:
	go tool cover -html="c.out"

lint:
	scopeguard ./... && golangci-lint run
