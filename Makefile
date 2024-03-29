TARGET=nightlight
SRCS=$(wildcard cmd/$(TARGET)/*.go) $(wildcard internal/*.go) $(wildcard internal/*.asm) \
     $(wildcard internal/*/*.go) $(wildcard internal/*/*.asm) \
     $(wildcard internal/*/*/*.go) $(wildcard internal/*/*/*.asm)
WEBSRCS=$(wildcard web/*) $(wildcard web/*/*) $(wildcard web/*/*/*)

BLOCKLY_UNPKG=https://unpkg.com/blockly@9.2.1/
BLOCKLY_SHORT=blockly.min.js javascript_compressed.js.map media/sprites.png media/click.mp3 media/disconnect.wav media/delete.mp3
BLOCKLY=$(patsubst %,web/blockly/%,$(BLOCKLY_SHORT))
BLOCKLY_WGET=-t 10 --retry-connrefused -nv 

FLAGS=-v -tags=jsoniter# -gcflags "-m"

GO=go

ifeq ($(OS),Windows_NT)
  EXECUTABLE=$(TARGET).exe
else
  EXECUTABLE=$(TARGET)
endif

all: $(EXECUTABLE)

install: $(EXECUTABLE)
	if [[ $< -nt /usr/local/bin/$< ]]; then sudo cp $< /usr/local/bin; fi

install-local: $(EXECUTABLE)
	cp $< ~/bin/

upgrade:
	go get -u ./...
	go mod tidy

web/blockly/%:
	mkdir -p $(@D) && wget -O $@ $(BLOCKLY_WGET) $(BLOCKLY_UNPKG)$*

$(EXECUTABLE): $(SRCS) $(BLOCKLY) $(WEBSRCS)
	$(GO) build -o $@ $(FLAGS) ./cmd/$(TARGET)

cross-platform: $(TARGET)_linux_amd64 $(TARGET)_linux_arm7 $(TARGET)_linux_arm64 $(TARGET)_darwin_amd64 $(TARGET)_darwin_arm64 $(TARGET)_windows_amd64.exe 

$(TARGET)_%_amd64: $(SRCS) $(BLOCKLY)
	GOOS=$* GOARCH=amd64 $(GO) build -o $@ $(FLAGS) ./cmd/$(TARGET)
	chmod a+x $@

$(TARGET)_%_amd64.exe: $(SRCS) $(BLOCKLY)
	GOOS=$* GOARCH=amd64 $(GO) build -o $@ $(FLAGS)  ./cmd/$(TARGET)
	chmod a+x $@

$(TARGET)_%_arm7: $(SRCS) $(BLOCKLY)
	GOOS=$* GOARCH=arm GOARM=7 $(GO) build -o $@ $(FLAGS)  ./cmd/$(TARGET)
	chmod a+x $@

$(TARGET)_%_arm64: $(SRCS) $(BLOCKLY)
	GOOS=$* GOARCH=arm64 $(GO) build -o $@ $(FLAGS)  ./cmd/$(TARGET)
	chmod a+x $@

tests:
	$(GO) test ./...

vulncheck:
	govulncheck ./...

clean:
	rm -f $(EXECUTABLE) $(TARGET)_*_amd64* $(TARGET)_*_amd64.exe 

realclean: clean
	rm -f $(BLOCKLY)
