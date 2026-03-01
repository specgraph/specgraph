// Copyright (c) 2026 Sean Bartell. All rights reserved.
// Use of this source code is governed by the MIT license
// that can be found in the LICENSE file.

package main

import (
	"fmt"
	"io"
)

// tableWriter wraps an io.Writer and captures the first write error,
// avoiding unchecked fmt.Fprint* calls when writing tabular output.
type tableWriter struct {
	w   io.Writer
	err error
}

func (tw *tableWriter) printf(format string, a ...any) {
	if tw.err != nil {
		return
	}
	_, tw.err = fmt.Fprintf(tw.w, format, a...)
}

func (tw *tableWriter) println(a ...any) {
	if tw.err != nil {
		return
	}
	_, tw.err = fmt.Fprintln(tw.w, a...)
}
