package pomo

import (
	"bytes"
	"fmt"
	"io"
	"log"
)

// logBuf collects log messages during a session so they don't
// corrupt the termui display. They get flushed to stderr on exit.
var logBuf bytes.Buffer

// InitLogger routes the standard logger into an in-memory buffer.
// Call once at program start, before any TUI takes over the terminal.
func InitLogger() {
	log.SetOutput(&logBuf)
	log.SetFlags(log.LstdFlags | log.Lmsgprefix)
}

// FlushLog writes any buffered log lines to w (typically os.Stderr)
// and clears the buffer. Safe to call multiple times — no-op when empty.
func FlushLog(w io.Writer) {
	if logBuf.Len() == 0 {
		return
	}
	_, _ = fmt.Fprint(w, logBuf.String())
	logBuf.Reset()
}
