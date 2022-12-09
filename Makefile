.PHONY: build test templates yamls
.FORCE:

GO_CMD ?= go
GO_FMT ?= gofmt

VERSION := $(shell git describe --tags --dirty --always)

build:
	@mkdir -p bin
	$(GO_CMD) build -v -o bin $(LDFLAGS) ./cmd/...

install:
	$(GO_CMD) install -v $(LDFLAGS) ./cmd/...

gofmt:
	@$(GO_FMT) -w -l $$(find . -name '*.go')

gofmt-verify:
	@out=`$(GO_FMT) -w -l -d $$(find . -name '*.go')`; \
	if [ -n "$$out" ]; then \
	    echo "$$out"; \
	    exit 1; \
	fi

lint:
	golint -set_exit_status ./...

test:
	$(GO_CMD) test ./cmd/...

