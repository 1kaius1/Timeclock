
APP_NAME := timeclock
BIN_DIR := bin
LINUX_BIN := $(BIN_DIR)/$(APP_NAME)-linux-amd64

.PHONY: all clean build-linux deb deps

all: build-linux

clean:
    rm -rf $(BIN_DIR)

# Install build dependencies (Debian/Ubuntu)
deps:
    sudo apt-get update && sudo apt-get install -y \
      build-essential pkg-config libgl1-mesa-dev xorg-dev \
      libx11-dev libxcursor-dev libxrandr-dev libxinerama-dev libxi-dev \
      libwayland-dev libxkbcommon-dev

build-linux:
    mkdir -p $(BIN_DIR)
    CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build -o $(LINUX_BIN) ./cmd/timeclock

deb: build-linux
    mkdir -p packaging/debian/usr/bin
    cp $(LINUX_BIN) packaging/debian/usr/bin/timeclock
    dpkg-deb --build packaging/debian timeclock_1.0.0_amd64.deb
    @echo "Built timeclock_1.0.0_amd64.deb"


