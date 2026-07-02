run:
	go run ./main.go

test:
	ENV=test go test ./...

migrations:
	go run cmd/migrations/run.go

proto:
	protoc \
		--go_out=shared/pb \
		--go_opt=paths=source_relative \
		--go-grpc_out=shared/pb \
		--go-grpc_opt=paths=source_relative \
		-I proto \
		proto/user/user.proto