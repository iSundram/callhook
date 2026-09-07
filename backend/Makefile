.PHONY: build run test vet docker clean

build:
	go build -o bin/callhook ./cmd/callhook
	go build -o bin/callhookctl ./cmd/callhookctl

run:
	go run ./cmd/callhook

# Dry-run demo server: no API key, short retry delay, fast scheduler tick.
demo:
	CALLHOOK_RETRY_DELAY=5s CALLHOOK_RETRY_TICK=1s go run ./cmd/callhook

test:
	go test ./...

vet:
	go vet ./...

docker:
	docker build -t callhook:latest .

clean:
	rm -rf bin data

# Build everything: web app + embed into the single Go binary.
full:
	cd ../web && npm ci --no-audit --no-fund && npm run build
	rm -rf internal/api/webdist && mkdir -p internal/api/webdist
	cp -r ../web/dist/* internal/api/webdist/
	touch internal/api/webdist/.gitkeep
	go build -o bin/callhook ./cmd/callhook
	go build -o bin/callhookctl ./cmd/callhookctl
