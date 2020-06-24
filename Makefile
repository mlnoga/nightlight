TARGET=nightlight
SRCS=$(wildcard cmd/$(TARGET)/*.go) $(wildcard internal/*.go) $(wildcard internal/*.asm)
FLAGS=-v# -gcflags "-m"

ifeq ($(OS),Windows_NT)
  EXECUTABLE=$(TARGET).exe
else
  EXECUTABLE=$(TARGET)
endif

all: $(EXECUTABLE)

install: $(EXECUTABLE)
	cp $< /usr/local/bin

$(EXECUTABLE): $(SRCS)
	go build -o $@ $(FLAGS) ./cmd/$(TARGET)

cross-platform: $(TARGET)_linux_amd64 $(TARGET)_darwin_amd64 $(TARGET)_windows_amd64.exe $(TARGET)_linux_arm7

$(TARGET)_%_amd64: $(SRCS)
	GOOS=$* GOARCH=amd64 go build -o $@ $(FLAGS) ./cmd/$(TARGET)
	chmod a+x $@

$(TARGET)_%_amd64.exe: $(SRCS)
	GOOS=$* GOARCH=amd64 go build -o $@ $(FLAGS)  ./cmd/$(TARGET)
	chmod a+x $@

$(TARGET)_%_arm7: $(SRCS)
	GOOS=$* GOARCH=arm GOARM=7 go build -o $@ $(FLAGS)  ./cmd/$(TARGET)
	chmod a+x $@

test:
	go test -v ./cmd/$(TARGET) ./internal

clean:
	rm -f $(EXECUTABLE) $(TARGET)_*_amd64* $(TARGET)_*_amd64.exe 