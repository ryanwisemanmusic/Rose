BINARY=rose
INSTALL_DIR=/usr/local/bin
LDFLAGS=-X 'github.com/ryanwi/rose/workspace.BuildRoseRoot=$(CURDIR)'

build:
	COPYFILE_DISABLE=1 COPY_EXTENDED_ATTRIBUTES_DISABLE=1 APPLEDOUBLE_DISABLE=1 go build -ldflags "$(LDFLAGS)" -o $(BINARY) .
	rm -f ._$(BINARY)

install: build
	cp $(BINARY) $(INSTALL_DIR)/$(BINARY)
	@echo "Installed $(BINARY) to $(INSTALL_DIR)/$(BINARY)"

uninstall:
	rm -f $(INSTALL_DIR)/$(BINARY)
	@echo "Removed $(BINARY) from $(INSTALL_DIR)/$(BINARY)"

run: build
	./$(BINARY)

clean:
	rm -f $(BINARY)

.PHONY: build install uninstall run clean
