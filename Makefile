BINARY_NAME=jcore
PREFIX=$(HOME)/.local

BASH_COMPLETION_DIR=$(HOME)/.local/share/bash-completion/completions
ZSH_COMPLETION_DIR=$(HOME)/.local/share/zsh/site-functions
FISH_COMPLETION_DIR=$(HOME)/.config/fish/completions

build:
	go build -o bin/${BINARY_NAME} ./cmd/jcore

clean:
	rm -f bin/${BINARY_NAME}

test:
	go test ./...

run:
	go run ./cmd/jcore

install: build
	mkdir -p $(PREFIX)/bin
	cp bin/${BINARY_NAME} $(PREFIX)/bin/${BINARY_NAME}
	$(MAKE) install-completions

install-completions: build
	mkdir -p $(BASH_COMPLETION_DIR) $(ZSH_COMPLETION_DIR) $(FISH_COMPLETION_DIR)
	./bin/${BINARY_NAME} completion bash > $(BASH_COMPLETION_DIR)/${BINARY_NAME}
	./bin/${BINARY_NAME} completion zsh > $(ZSH_COMPLETION_DIR)/_${BINARY_NAME}
	./bin/${BINARY_NAME} completion fish > $(FISH_COMPLETION_DIR)/${BINARY_NAME}.fish
