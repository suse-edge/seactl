BINARY_NAME=seactl
VERSION ?= 1.5.1
LDFLAGS = -ldflags "-X main.version=$(VERSION)"

build:
	go build $(LDFLAGS) -o seactl .

run:
	go run $(LDFLAGS) main.go


compile:
	echo "Compiling for every OS and Platform"
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o ${BINARY_NAME}-x86 .
	GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -o ${BINARY_NAME}-aarch64 .
	#GOOS=freebsd GOARCH=386 go build -o ${BINARY_NAME} .

tag:
	git tag -a v$(VERSION) -m "v$(VERSION)"

test:
	go test -v ./... -cover

all: test build compile run
