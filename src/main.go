package main

import (
	"fmt"
	"io"
	"os"

	"github.com/sommerfeld-io/fantasy-hockey/internal/clock"
)

// run prints the current date and time, as returned by now, to out.
// It is injected into main so that tests can replace out and now with fakes.
func run(out io.Writer, now func() string) {
	fmt.Fprintln(out, now())
}

func main() {
	run(os.Stdout, clock.Now)
}
