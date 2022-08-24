## Process example

#### Service Mode
```
$go run golib-proc.go
I1104 12:32:47.983760   90011 golib-proc.go:42] entering start()
I1104 12:32:47.983873   90011 golib-proc.go:44] Press ctrl-c to leave process
^CI1104 12:33:14.245716   90011 proc.go:275] See ya!
```

#### Command Mode
```
$go run golib-proc.go hello
I1104 12:33:50.889792   90370 golib-proc.go:54] hello
```
