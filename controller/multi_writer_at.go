package controller

import (
	"io"
	"strings"
	"sync"
)

type MultiWriterAt struct {
	writers  []io.WriterAt
	updaters []io.WriterAt
}

type MultiWriterError struct {
	Writers       []io.WriterAt
	Updaters      []io.WriterAt
	ReplicaErrors []error
	QuorumErrors  []error
}

func (m *MultiWriterError) Error() string {
	errors := []string{}
	for _, err := range m.ReplicaErrors {
		if err != nil {
			errors = append(errors, err.Error())
		}
	}

	for _, err := range m.QuorumErrors {
		if err != nil {
			errors = append(errors, err.Error())
		}
	}

	switch len(errors) {
	case 0:
		return "Unknown"
	case 1:
		return errors[0]
	default:
		return strings.Join(errors, "; ")
	}
}

func (m *MultiWriterAt) WriteAt(p []byte, off int64) (n int, err error) {
	quorumErrs := make([]error, len(m.writers))
	replicaErrs := make([]error, len(m.writers))
	quorumErrored := false
	replicaErrored := false
	wg := sync.WaitGroup{}
	var errors MultiWriterError

	for i, w := range m.writers {
		wg.Add(1)
		go func(index int, w io.WriterAt) {
			_, err := w.WriteAt(p, off)
			if err != nil {
				replicaErrored = true
				replicaErrs[index] = err
			}
			wg.Done()
		}(i, w)
	}
	for i, w := range m.updaters {
		wg.Add(1)
		go func(index int, w io.WriterAt) {
			_, err := w.WriteAt(nil, 0)
			if err != nil {
				quorumErrored = true
				quorumErrs[index] = err
			}
			wg.Done()
		}(i, w)
	}
	wg.Wait()
	if replicaErrored {
		errors.Writers = m.writers
		errors.ReplicaErrors = replicaErrs
	} else if quorumErrored {
		errors.Updaters = m.updaters
		errors.QuorumErrors = quorumErrs
	}

	if replicaErrored || quorumErrored {
		return 0, &errors
	}
	return len(p), nil
}
