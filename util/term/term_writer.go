/*
Copyright 2016 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package term

import (
	"bytes"
	"io"
	"os"
	"unicode"

	"github.com/yubo/golib/util/term/moby/term"
)

const nbsp = 0xA0

type wordWrapWriter struct {
	limit  uint
	writer io.Writer
}

// NewResponsiveWriter creates a Writer that detects the column width of the
// terminal we are in, and adjusts every line width to fit and use recommended
// terminal sizes for better readability. Does proper word wrapping automatically.
//    if terminal width >= 120 columns		use 120 columns
//    if terminal width >= 100 columns		use 100 columns
//    if terminal width >=  80 columns		use  80 columns
// In case we're not in a terminal or if it's smaller than 80 columns width,
// doesn't do any wrapping.
func NewResponsiveWriter(w io.Writer) io.Writer {
	file, ok := w.(*os.File)
	if !ok {
		return w
	}
	fd := file.Fd()
	if !term.IsTerminal(fd) {
		return w
	}

	terminalSize := GetSize(fd)
	if terminalSize == nil {
		return w
	}

	var limit uint
	switch {
	case terminalSize.Width >= 120:
		limit = 120
	case terminalSize.Width >= 100:
		limit = 100
	case terminalSize.Width >= 80:
		limit = 80
	}

	return NewWordWrapWriter(w, limit)
}

// NewWordWrapWriter is a Writer that supports a limit of characters on every line
// and does auto word wrapping that respects that limit.
func NewWordWrapWriter(w io.Writer, limit uint) io.Writer {
	return &wordWrapWriter{
		limit:  limit,
		writer: w,
	}
}

func (w wordWrapWriter) Write(p []byte) (nn int, err error) {
	if w.limit == 0 {
		return w.writer.Write(p)
	}
	original := string(p)
	wrapped := WrapString(original, w.limit)
	return w.writer.Write([]byte(wrapped))
}

// NewPunchCardWriter is a NewWordWrapWriter that limits the line width to 80 columns.
func NewPunchCardWriter(w io.Writer) io.Writer {
	return NewWordWrapWriter(w, 80)
}

type maxWidthWriter struct {
	maxWidth     uint
	currentWidth uint
	written      uint
	writer       io.Writer
}

// NewMaxWidthWriter is a Writer that supports a limit of characters on every
// line, but doesn't do any word wrapping automatically.
func NewMaxWidthWriter(w io.Writer, maxWidth uint) io.Writer {
	return &maxWidthWriter{
		maxWidth: maxWidth,
		writer:   w,
	}
}

func (m maxWidthWriter) Write(p []byte) (nn int, err error) {
	for _, b := range p {
		if m.currentWidth == m.maxWidth {
			m.writer.Write([]byte{'\n'})
			m.currentWidth = 0
		}
		if b == '\n' {
			m.currentWidth = 0
		}
		_, err := m.writer.Write([]byte{b})
		if err != nil {
			return int(m.written), err
		}
		m.written++
		m.currentWidth++
	}
	return len(p), nil
}

// WrapString wraps the given string within lim width in characters.
//
// Wrapping is currently naive and only happens at white-space. A future
// version of the library will implement smarter wrapping. This means that
// pathological cases can dramatically reach past the limit, such as a very
// long word.
func WrapString(s string, lim uint) string {
	// Initialize a buffer with a slightly larger size to account for breaks
	init := make([]byte, 0, len(s))
	buf := bytes.NewBuffer(init)

	var current uint
	var wordBuf, spaceBuf bytes.Buffer
	var wordBufLen, spaceBufLen uint

	for _, char := range s {
		if char == '\n' {
			if wordBuf.Len() == 0 {
				if current+spaceBufLen > lim {
					current = 0
				} else {
					current += spaceBufLen
					spaceBuf.WriteTo(buf)
				}
				spaceBuf.Reset()
				spaceBufLen = 0
			} else {
				current += spaceBufLen + wordBufLen
				spaceBuf.WriteTo(buf)
				spaceBuf.Reset()
				spaceBufLen = 0
				wordBuf.WriteTo(buf)
				wordBuf.Reset()
				wordBufLen = 0
			}
			buf.WriteRune(char)
			current = 0
		} else if unicode.IsSpace(char) && char != nbsp {
			if spaceBuf.Len() == 0 || wordBuf.Len() > 0 {
				current += spaceBufLen + wordBufLen
				spaceBuf.WriteTo(buf)
				spaceBuf.Reset()
				spaceBufLen = 0
				wordBuf.WriteTo(buf)
				wordBuf.Reset()
				wordBufLen = 0
			}

			spaceBuf.WriteRune(char)
			spaceBufLen++
		} else {
			wordBuf.WriteRune(char)
			wordBufLen++

			if current+wordBufLen+spaceBufLen > lim && wordBufLen < lim {
				buf.WriteRune('\n')
				current = 0
				spaceBuf.Reset()
				spaceBufLen = 0
			}
		}
	}

	if wordBuf.Len() == 0 {
		if current+spaceBufLen <= lim {
			spaceBuf.WriteTo(buf)
		}
	} else {
		spaceBuf.WriteTo(buf)
		wordBuf.WriteTo(buf)
	}

	return buf.String()
}
