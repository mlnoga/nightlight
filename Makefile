TARGET=nightlight
SRCS=$(wildcard cmd/$(TARGET)/*.go) $(wildcard internal/*.go) $(wildcard internal/*.asm) \
     $(wildcard internal/*/*.go) $(wildcard internal/*/*.asm) \
     $(wildcard internal/*/*/*.go) $(wildcard internal/*/*/*.asm)
BLOCKLY=web/blockly/blockly_compressed.js
FLAGS=-v -tags=jsoniter# -gcflags "-m"

ifeq ($(OS),Windows_NT)
  EXECUTABLE=$(TARGET).exe
else
  EXECUTABLE=$(TARGET)
endif

all: $(EXECUTABLE)

install: $(EXECUTABLE)
	cp $< /usr/local/bin

install-local: $(EXECUTABLE)
	cp $< ~/bin/

$(BLOCKLY):
	cd web && git clone https://github.com/google/blockly.git && cd ..

$(EXECUTABLE): $(SRCS) $(BLOCKLY)
	go build -o $@ $(FLAGS) ./cmd/$(TARGET)

cross-platform: $(TARGET)_linux_amd64 $(TARGET)_darwin_amd64 $(TARGET)_windows_amd64.exe $(TARGET)_linux_arm7 $(TARGET)_linux_arm64

$(TARGET)_%_amd64: $(SRCS) $(BLOCKLY)
	GOOS=$* GOARCH=amd64 go build -o $@ $(FLAGS) ./cmd/$(TARGET)
	chmod a+x $@

$(TARGET)_%_amd64.exe: $(SRCS) $(BLOCKLY)
	GOOS=$* GOARCH=amd64 go build -o $@ $(FLAGS)  ./cmd/$(TARGET)
	chmod a+x $@

$(TARGET)_%_arm7: $(SRCS) $(BLOCKLY)
	GOOS=$* GOARCH=arm GOARM=7 go build -o $@ $(FLAGS)  ./cmd/$(TARGET)
	chmod a+x $@

$(TARGET)_%_arm64: $(SRCS) $(BLOCKLY)
	GOOS=$* GOARCH=arm64 go build -o $@ $(FLAGS)  ./cmd/$(TARGET)
	chmod a+x $@

test:
	go test -v ./cmd/$(TARGET) ./internal

clean:
	rm -f $(EXECUTABLE) $(TARGET)_*_amd64* $(TARGET)_*_amd64.exe 
