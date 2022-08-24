# stream

## record

This spawns a new shell instance and records all terminal output. When you're ready to finish simply exit the shell either by hitting Ctrl-D.

```sh
$ go run ./record/main.go --file ./testdata/example.rec
```

## player

Read and play the recorded file

```sh
$ go run ./player/main.go --file ./testdata/example.rec
```

## convert

convert rec into asciinema format

```sh
$ go run ./convert/main.go --in ./testdata/example.rec --out ./testdata/example.cast
```

asciinema usage - https://asciinema.org/

```sh
# play terminal session
$ asciinema play testdata/demo.cast

# upload locally saved terminal session to asciinema.org
$ asciinema upload testdata/demo.cast
```
