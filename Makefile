PACKAGE := goqoa

# Go defintions
GOCMD ?= go
GOBUILD := $(GOCMD) build
GOINSTALL := $(GOCMD) install
GORUN := $(GOCMD) run
GOARCH := amd64

# Build definitions
BUILD_ENTRY := $(PWD)
BIN_DIR := $(PWD)/bin

# Determine the file extension based on the platform
ifeq ($(OS),Windows_NT)
  EXTENSION := .exe
else
  EXTENSION :=
endif
# Different platform support
PLATFORMS := linux windows darwin
BINARIES := $(addprefix $(BIN_DIR)/,$(addsuffix /$(PACKAGE)$(EXTENSION),$(PLATFORMS)))

# Fancy colors
BOLD := $(shell tput bold)
ITALIC := $(shell tput dim)
YELLOW := $(shell tput setaf 222)
GREEN := $(shell tput setaf 114)
BLUE := $(shell tput setaf 111)
PURPLE := $(shell tput setaf 183)
END := $(shell tput sgr0)

# Function to colorize a command help string
command-style = $(GREEN)$(BOLD)$1$(END)  $(ITALIC)$(BLUE)$2$(END)

define help_text
$(PURPLE)$(BOLD)Targets:$(END)
  - $(call command-style,all,    Build $(PACKAGE) for all targets (Linux, Windows, Mac, 64-bit))
  - $(call command-style,build,  Build $(PACKAGE) for current host architecture)
  - $(call command-style,install,Build and install $(PACKAGE) for current host)
  - $(call command-style,test,   Run all tests)
  - $(call command-style,clean,  Delete built artifacts)
  - $(call command-style,[help], Print this help)
endef
export help_text

.PHONY: test clean help build all install run debug

# Determine OS-specific echo command
OS := $(shell uname)
ifeq ($(OS),Darwin)
  # macOS doesn't need -e flag
  ECHO := echo
else
  # Linux and others need -e flag to interpret escape sequences
  ECHO := echo -e
endif

help:
	@$(ECHO) "$$help_text"

# Select the right binary for the current host
ifeq ($(OS),Windows_NT)
  BIN := $(BIN_DIR)/windows/$(PACKAGE)$(EXTENSION)
else
  UNAME := $(shell uname -s)
  ifeq ($(UNAME),Linux)
    BIN := $(BIN_DIR)/linux/$(PACKAGE)
  endif
  ifeq ($(UNAME),Darwin)
    BIN := $(BIN_DIR)/darwin/$(PACKAGE)
  endif
endif

SOURCES := $(shell find . -name "*.go")
SOURCES += go.mod go.sum

all: $(BINARIES)
	@$(ECHO) "$(GREEN)📦️ Builds are complete: $(END)$(PURPLE)$(BIN_DIR)$(END)"

$(BIN_DIR)/%/$(PACKAGE)$(EXTENSION): $(SOURCES)
	@$(ECHO) "$(YELLOW)🚧 Building $@...$(END)"
	@CGO_ENABLED=1 GOARCH=$(GOARCH) GOOS=$* $(GOBUILD) -o $@ $(BUILD_ENTRY)

build: $(BIN)
	@$(ECHO) "$(GREEN)📦️ Build is complete: $(END)$(PURPLE)$(BIN)$(END)"

clean:
	@rm -rf $(BIN_DIR)
	@$(ECHO) "$(GREEN)Cleaned!$(END)"

TEST_FILES = $(PWD)/pkg/qoa/
test:
	@$(ECHO) "$(YELLOW)Testing...$(END)"
	@go test $(TEST_FILES)
	@$(ECHO) "$(GREEN)✅ Test is complete!$(END)"

install: $(BIN)
	@$(ECHO) "$(YELLOW)🚀 Installing $(BIN) to appropriate location...$(END)"
	@$(GOINSTALL) $(BUILD_ENTRY)
	@$(ECHO) "$(GREEN)✅ Installation complete!$(END)"

playrun:
	@$(GORUN) . play four_tet_baby.qoa
