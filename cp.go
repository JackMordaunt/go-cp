package cp

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/pkg/errors"
	"github.com/spf13/afero"
)

// Copier copies files concurrently.
// Safe to use as-is with sane defaults.
type Copier struct {
	// Fs is the filesystem object to operate on. Defaults to `afero.OsFs`.
	Fs afero.Fs
	// Clobber is whether or not to copy into a directory that already
	// exists, potentially clobbering any files.
	Clobber bool
	// Parallel is the number of parallel workers to use.
	// Higher means better throughput. You will need to respect your OS's
	// open file descriptor maximum.
	Parallel int

	// seen tracks the file paths already copied to.
	seen *sync.Map
}

// Copy executes the copy.
// Safe for conccurent use.
func (c *Copier) Copy(from, to string) error {
	if from == to {
		return nil
	}
	if c.Fs == nil {
		c.Fs = afero.NewOsFs()
	}
	fromFi, err := c.Fs.Stat(from)
	if err != nil {
		return errors.Wrap(err, "reading file metadata")
	}
	_, err = c.Fs.Stat(to)
	if !os.IsNotExist(err) && !c.Clobber {
		return ErrClobberAvoided{to}
	}
	if !fromFi.IsDir() {
		return copyFile(c.Fs, from, to)
	}
	if err := c.Fs.MkdirAll(to, fromFi.Mode()); err != nil {
		return err
	}
	if c.seen == nil {
		c.seen = &sync.Map{}
	}
	return c.copy(from, to)
}

func copyFile(fs afero.Fs, from, to string) error {
	fromFile, err := fs.Open(from)
	if err != nil {
		return errors.Wrapf(err, "opening %s", from)
	}
	defer fromFile.Close()
	fromFi, err := fromFile.Stat()
	if err != nil {
		return errors.Wrap(err, "reading file metadata")
	}
	if err := fs.MkdirAll(filepath.Dir(to), fromFi.Mode()); err != nil {
		return errors.Wrapf(err, "preparing directories for %s", to)
	}
	toFile, err := fs.OpenFile(to, os.O_CREATE|os.O_RDWR, fromFi.Mode())
	if err != nil {
		return errors.Wrapf(err, "creating %s", to)
	}
	defer toFile.Close()
	if _, err := io.Copy(toFile, fromFile); err != nil {
		return errors.Wrapf(err, "copying file from %s to %s", from, to)
	}
	return nil
}

// copy copies an entire directory concurrently.
func (c *Copier) copy(from, to string) error {
	cp := &copier{
		fs:       c.Fs,
		parallel: c.Parallel,
		seen:     c.seen,
		work:     make(chan job),
		failures: make(chan error),
	}
	return cp.copy(from, to)
}

// copier private type which implements the concurrency.
type copier struct {
	fs       afero.Fs
	parallel int
	seen     *sync.Map
	work     chan job
	failures chan error
}

func (c copier) copy(from, to string) error {
	go c.walk(from, to)
	go c.copyFiles()
	return c.collectErrors()
}

func (c *copier) copyFiles() {
	if c.parallel < 1 {
		c.parallel = 10
	}
	jobs := &sync.WaitGroup{}
	for ii := 0; ii < c.parallel-1; ii++ {
		jobs.Add(1)
		go func() {
			for job := range c.work {
				if err := copyFile(
					c.fs,
					job.From,
					job.To,
				); err != nil {
					c.failures <- err
				}
			}
			jobs.Done()
		}()
	}
	jobs.Wait()
	close(c.failures)
}

func (c *copier) collectErrors() error {
	var errs []error
	for err := range c.failures {
		errs = append(errs, err)
	}
	if len(errs) > 0 {
		return Failures{errs}
	}
	return nil
}

func (c *copier) walk(from, to string) {
	walker := func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		toPath := filepath.Join(to, strings.Replace(path, from, "", 1))
		if _, ok := c.seen.Load(toPath); ok {
			return nil
		}
		c.seen.Store(toPath, struct{}{})
		c.work <- job{
			From: path,
			To:   toPath,
		}
		return nil
	}
	if err := afero.Walk(c.fs, from, walker); err != nil {
		c.failures <- errors.Wrap(err, "walking file system")
	}
	close(c.work)
}

type job struct {
	From, To string
}

// Failures wraps a list of errors.
type Failures struct {
	list []error
}

func (err Failures) Error() string {
	b := &strings.Builder{}
	b.WriteString("[")
	for ii, failure := range err.list {
		b.WriteString(failure.Error())
		if ii != len(err.list)-1 {
			b.WriteString(",\n")
		}
	}
	b.WriteString("\n]")
	return b.String()
}

// ErrClobberAvoided describes an attempt to overwrite an existing file.
type ErrClobberAvoided struct {
	Path string
}

func (err ErrClobberAvoided) Error() string {
	return fmt.Sprintf("avoided attempt to clobber existing file or directory %q",
		err.Path)
}
