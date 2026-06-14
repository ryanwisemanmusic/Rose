BINARY=rose
INSTALL_DIR=/usr/local/bin
LDFLAGS=-X 'github.com/ryanwi/rose/workspace.BuildRoseRoot=$(CURDIR)'

build:
	COPYFILE_DISABLE=1 COPY_EXTENDED_ATTRIBUTES_DISABLE=1 APPLEDOUBLE_DISABLE=1 go build -ldflags "$(LDFLAGS)" -o $(BINARY) .
	rm -f ._$(BINARY)

stop:
	@echo "Stopping existing $(BINARY) processes..."
	@pkill -TERM -x $(BINARY) 2>/dev/null || true
	@sleep 0.3
	@pkill -KILL -x $(BINARY) 2>/dev/null || true

force-stop:
	@echo "Force-stopping existing $(BINARY) processes..."
	@pkill -KILL -x $(BINARY) 2>/dev/null || true

install: stop build
	install -m 0755 $(BINARY) $(INSTALL_DIR)/$(BINARY)
	xattr -c $(INSTALL_DIR)/$(BINARY) 2>/dev/null || true
	xattr -d com.apple.provenance $(INSTALL_DIR)/$(BINARY) 2>/dev/null || true
	@echo "Installed $(BINARY) to $(INSTALL_DIR)/$(BINARY)"

uninstall:
	rm -f $(INSTALL_DIR)/$(BINARY)
	@echo "Removed $(BINARY) from $(INSTALL_DIR)/$(BINARY)"

run: stop build
	./$(BINARY)

clean:
	rm -f $(BINARY)

.PHONY: build stop force-stop install uninstall run clean
