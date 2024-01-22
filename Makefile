build-cli:
	go build -o bin/kasa-exporter

build-cli-arm:
	GOOS=linux GOARCH=arm64 go build -o bin/kasa-exporter-arm

run:
	go run main.go