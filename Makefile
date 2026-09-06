.PHONY: build run test vet docker clean

build:
	go build -o bin/calle ./cmd/calle
	go build -o bin/callectl ./cmd/callectl

run:
	go run ./cmd/calle

# Dry-run demo server: no API key, short retry delay, fast scheduler tick.
demo:
	CALLE_RETRY_DELAY=5s CALLE_RETRY_TICK=1s go run ./cmd/calle

test:
	go test ./...

vet:
	go vet ./...

docker:
	docker build -t calle:latest .

clean:
	rm -rf bin data
