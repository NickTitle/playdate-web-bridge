.PHONY: build audio test
build:
	mkdir -p build
	go build -o build/playdate-web-bridge ./cmd/playdate-web-bridge
audio:
	mkdir -p build
	$(CC) -shared -fPIC -Wall -Wextra -Werror -o build/audio_stream.so native/audio_stream.c -ldl
test:
	go test -race ./...
	node --test tests/*.test.mjs
