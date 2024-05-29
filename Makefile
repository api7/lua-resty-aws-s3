INST_PREFIX ?= /usr

ifeq ($(shell uname),Darwin)
    ifeq ($(shell uname -m),arm64)
        INST_PREFIX = /opt/homebrew
				OS = darwin
				ARCH = arm64
    endif
endif

INST_LIBDIR ?= $(INST_PREFIX)/lib/lua/5.1
INST_LUADIR ?= $(INST_PREFIX)/share/lua/5.1
INSTALL ?= install

# Specify the output shared library name
OUTPUT_NAME ?= s3.so

.PHONY: install
install:
	$(INSTALL) -d $(INST_LUADIR)/resty/
	$(INSTALL) s3.lua $(INST_LUADIR)/resty/
	cd go-src && CGO_ENABLED=1 GOOS=$(OS) GOARCH=$(ARCH) go build -o $(OUTPUT_NAME) -buildmode=c-shared main.go
