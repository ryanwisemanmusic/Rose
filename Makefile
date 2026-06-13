BINARY=rose
INSTALL_DIR=/usr/local/bin

build:
	go build -o $(BINARY) .

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
