all: example

example: example.go ../*.go
	go build -o example example.go

t1: example
	./example  --address=1.1.1.1

t2: example
	SERVER_NAME=a.com ./example

help: example
	./example -h
