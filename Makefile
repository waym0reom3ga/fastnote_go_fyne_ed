.PHONY: build selftest clicks test clean

BIN := fastnotes

build:
	go build -o $(BIN) .

selftest: build
	./$(BIN) --headless --notes-dir /tmp/fastnote_notes --selftest

clicks: build
	go test -count=1 -run TestUI .

test: selftest clicks

clean:
	rm -f $(BIN)