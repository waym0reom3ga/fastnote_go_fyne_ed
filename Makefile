.PHONY: build test-ui test clean

BIN := fastnotes

build:
	go build -o $(BIN) .

test-ui: build
	go test -count=1 -run TestUI .

test: build
	./$(BIN) --version
	$(MAKE) test-ui

clean:
	rm -f $(BIN)