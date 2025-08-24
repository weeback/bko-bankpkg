package log

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type FileWriter interface {
	Write(p []byte) (n int, err error)
	SyncFile(path string)
	Close()
}

func NewFile() FileWriter {
	return &file{
		Buffer: bytes.Buffer{},
	}
}

type file struct {
	bytes.Buffer
}

func (f *file) Write(p []byte) (n int, err error) {
	if !bytes.HasSuffix(p, []byte("\n")) {
		p = append(p, '\n')
	}
	return f.Buffer.Write(p)
}

// Close: reset buffer
func (f *file) Close() {
	defer f.Reset()
	// print buffer to stdout if cannot create file
	if _, err := io.Copy(os.Stdout, &f.Buffer); err != nil {
		log.Printf("print log to stdout error: %v", err)
	}
}

func (f *file) SyncFile(path string) {
	// split path to dir and filename
	dir, filename := filepath.Split(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		log.Printf("create log directory error: %v", err)
	}
	// get extension
	ext := filepath.Ext(filename)
	// format filename
	filename = fmt.Sprintf("%s_%s_%d%s",
		strings.TrimSuffix(filename, ext),
		time.Now().Format(time.DateOnly), time.Now().Unix(),
		ext)
	// reset buffer after end of writing
	defer f.Reset()
	// create file
	file, err := os.Create(filepath.Join(dir, filename))
	if err != nil {
		log.Printf("create log file error: %v", err)
		// print buffer to stdout if cannot create file
		if _, copyErr := io.Copy(os.Stdout, &f.Buffer); copyErr != nil {
			log.Printf("print log to stdout error: %v", copyErr)
		}
		return
	}
	defer file.Close()
	// write to file
	if _, err = file.Write(f.Bytes()); err != nil {
		log.Printf("write log file error: %v", err)
		// print buffer to stdout if cannot create file
		if _, copyErr := io.Copy(os.Stdout, &f.Buffer); copyErr != nil {
			log.Printf("print log to stdout error: %v", copyErr)
		}
		return
	}
}
