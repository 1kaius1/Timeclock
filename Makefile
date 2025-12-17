APP_NAME := timeclock
BIN_DIR := bin
LINUX_BIN := $(BIN_DIR)/$(APP_NAME)-linux-amd64
WINDOWS_BIN := $(BIN_DIR)/$(APP_NAME)-windows-amd64.exe
DARWIN_BIN := $(BIN_DIR)/$(APP_NAME)-darwin-arm64

.PHONY: all clean build-linux build-windows build-darwin deb deps-linux deps-win32 deps-darwin

all: build-linux

clean:
	rm -rf $(BIN_DIR)

# Install build dependencies for Linux native builds (Debian/Ubuntu)
deps-linux:
	sudo apt-get update && sudo apt-get install -y \
		build-essential pkg-config libgl1-mesa-dev xorg-dev \
		libx11-dev libxcursor-dev libxrandr-dev libxinerama-dev libxi-dev \
		libwayland-dev libxkbcommon-dev

# Install MinGW-w64 cross-compiler for Windows builds (Debian/Ubuntu)
deps-win32:
	sudo apt-get update && sudo apt-get install -y \
		gcc-mingw-w64-x86-64

# Install build dependencies for macOS native builds
deps-darwin:
	@echo "Installing macOS build dependencies..."
	@command -v brew >/dev/null 2>&1 || { echo "Homebrew not found. Install from https://brew.sh"; exit 1; }
	brew install pkg-config

build-linux:
	mkdir -p $(BIN_DIR)
	CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build -o $(LINUX_BIN) ./cmd/timeclock

build-windows:
	mkdir -p $(BIN_DIR)
	CGO_ENABLED=1 GOOS=windows GOARCH=amd64 CC=x86_64-w64-mingw32-gcc go build -o $(WINDOWS_BIN) ./cmd/timeclock

build-darwin:
	mkdir -p $(BIN_DIR)
	CGO_ENABLED=1 GOOS=darwin GOARCH=arm64 go build -o $(DARWIN_BIN) ./cmd/timeclock

deb: build-linux
	mkdir -p packaging/debian/usr/bin
	cp $(LINUX_BIN) packaging/debian/usr/bin/timeclock
	dpkg-deb --build packaging/debian timeclock_1.0.0_amd64.deb
	@echo "Built timeclock_1.0.0_amd64.deb"

