.PHONY: build install test run doctor clean

BINARY := marspi-cli

build:
	go build -o $(BINARY) .

install:
	go install .

test:
	go test ./...

run: build
	./$(BINARY)

doctor: build
	./$(BINARY) -doctor

clean:
	rm -f $(BINARY)
