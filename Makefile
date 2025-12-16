
APP_NAME := timeclock
BIN_DIR := bin
LINUX_BIN := $(BIN_DIR)/$(APP_NAME)-linux-amd64
DEB_DIR := packaging/debian

.PHONY: all clean build-linux deb

all: build-linux

clean:
    rm -rf $(BIN_DIR)
    rm -rf $(DEB_DIR)/usr/bin/$(APP_NAME)

build-linux:
    mkdir -p $(BIN_DIR)
    GOOS=linux GOARCH=amd64 go build -o $(LINUX_BIN) ./cmd/timeclock

# Build a simple .deb package (amd64). Requires dpkg-deb.
deb: build-linux
    # Install binary into staging tree
    mkdir -p $(DEB_DIR)/usr/bin
    cp $(LINUX_BIN) $(DEB_DIR)/usr/bin/$(APP_NAME)
    # Build the deb
    dpkg-deb --build packaging/debian $(APP_NAME)_1.0.0_amd64.deb
    @echo "Built $(APP_NAME)_1.0.0_amd64.deb"


