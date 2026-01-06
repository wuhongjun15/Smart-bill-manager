package services

import "io"

// ZipStream is a prepared ZIP export that can be written to an io.Writer (usually an HTTP response).
// This avoids buffering the entire ZIP in memory.
type ZipStream struct {
	Filename string
	Write    func(w io.Writer) error
}

