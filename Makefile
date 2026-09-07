BINARY_NAME=jcore
PREFIX=$(HOME)/.local

BASH_COMPLETION_DIR=$(HOME)/.local/share/bash-completion/completions
ZSH_COMPLETION_DIR=$(HOME)/.local/share/zsh/site-functions
FISH_COMPLETION_DIR=$(HOME)/.config/fish/completions

LDFLAGS := -s -w -X github.com/JCO-Digital/jcore/internal/config.AppVersion=$(shell git describe --tags --always --dirty || echo "dev")

# GOOS_GOARCH pairs published as release assets by .github/workflows/release.yml,
# matching the names update.AssetName() looks for.
RELEASE_TARGETS := linux_amd64 linux_arm64 darwin_amd64 darwin_arm64 windows_amd64

build:
	go build -ldflags "$(LDFLAGS)" -o bin/${BINARY_NAME} ./cmd/jcore

release: $(addprefix release-,$(RELEASE_TARGETS)) release-legacy-alias

release-%:
	$(eval GOOS := $(word 1,$(subst _, ,$*)))
	$(eval GOARCH := $(word 2,$(subst _, ,$*)))
	$(eval EXT := $(if $(filter windows,$(GOOS)),.exe,))
	GOOS=$(GOOS) GOARCH=$(GOARCH) go build -ldflags "$(LDFLAGS)" -o bin/${BINARY_NAME}_$(GOOS)_$(GOARCH)$(EXT) ./cmd/jcore

# Unsuffixed copy for the legacy TS CLI's self-update, which fetches a
# plain "jcore" asset with no OS/arch suffix and no signature check.
release-legacy-alias: release-linux_amd64
	cp bin/${BINARY_NAME}_linux_amd64 bin/${BINARY_NAME}

clean:
	rm -f bin/${BINARY_NAME} bin/${BINARY_NAME}_*

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
