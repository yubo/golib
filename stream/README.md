# stream

[![demo](https://asciinema.org/a/Ff5lHekWWExYdaCipYaULG1eM.svg)](https://asciinema.org/a/Ff5lHekWWExYdaCipYaULG1eM?autoplay=1)


```
tty(TtyStreams) -->   pty(PtyStreams)
In(Reader)      -->   In(Writer)
Out(Writer)     <--   Out(Reader)
In(Writer)      <--   ErrOut(Reader)
```

