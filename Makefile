build/hypeliberator-go-linux-amd64:
	GOOS=linux GOARCH=amd64 go build -v -o build/hypeliberator-go-linux-amd64

.PHONY: build/hypeliberator-go-linux-amd64
