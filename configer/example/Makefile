all: example

example: example.go ../*.go
	go build -o example example.go

t1: example
	./example  --address=1.1.1.1 -f ./example.yaml --timeout=5

t2: example
	SERVER_NAME=a.com ./example

t3: example
	./example --timeout=10

help: example
	./example -h
