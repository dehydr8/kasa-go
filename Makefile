build-cli:
	go build -o bin/kasa-exporter main.go

build-cli-arm:
	GOOS=linux GOARCH=arm64 go build -o bin/kasa-exporter-arm main.go

run:
	go run main.go