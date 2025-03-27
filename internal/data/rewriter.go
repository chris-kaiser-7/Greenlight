package data

import (
	"io"
)

type Rewriter struct {
	Writer io.Writer
}

func (r Rewriter) Write(p []byte) (n int, err error) {
	return r.Writer.Write(p)
}

//the diffrence between using a reciver and not a reciver causes this to work or not?
