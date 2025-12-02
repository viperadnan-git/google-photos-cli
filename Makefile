.PHONY: build clean

build:
	go build -o gpcli .

clean:
	rm -f gpcli
