BINARY_NAME = volume-anomaly-tester

.PHONY: build clean

build:
	go build -buildvcs=false -o $(BINARY_NAME) volume-anomaly-tester.go

clean:
	rm -f $(BINARY_NAME)