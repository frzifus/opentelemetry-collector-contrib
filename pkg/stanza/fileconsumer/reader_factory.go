// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package fileconsumer // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer"

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"go.opentelemetry.io/collector/extension/experimental/storage"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/fingerprint"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/util"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/pipeline"
)

type readerFactory struct {
	*zap.SugaredLogger
	readerConfig    *readerConfig
	fromBeginning   bool
	splitterFactory splitterFactory
	encodingConfig  helper.EncodingConfig
	headerSettings  *headerSettings
}

func (f *readerFactory) newReader(file *os.File, fp *fingerprint.Fingerprint) (*Reader, error) {
	return f.newReaderBuilder().
		withFile(file).
		withFingerprint(fp).
		build()
}

// copy creates a deep copy of a Reader
func (f *readerFactory) copy(old *Reader, newFile *os.File) (*Reader, error) {
	return f.newReaderBuilder().
		withFile(newFile).
		withFingerprint(old.Fingerprint.Copy()).
		withOffset(old.Offset).
		withSplitterFunc(old.lineSplitFunc).
		withFileAttributes(util.MapCopy(old.FileAttributes)).
		withHeaderFinalized(old.HeaderFinalized).
		build()
}

func (f *readerFactory) unsafeReader() (*Reader, error) {
	return f.newReaderBuilder().build()
}

func (f *readerFactory) newFingerprint(file *os.File) (*fingerprint.Fingerprint, error) {
	return fingerprint.New(file, f.readerConfig.fingerprintSize)
}

type readerBuilder struct {
	*readerFactory
	file            *os.File
	fp              *fingerprint.Fingerprint
	offset          int64
	splitFunc       bufio.SplitFunc
	headerFinalized bool
	fileAttributes  map[string]any
}

func (f *readerFactory) newReaderBuilder() *readerBuilder {
	return &readerBuilder{readerFactory: f, fileAttributes: map[string]any{}}
}

func (b *readerBuilder) withSplitterFunc(s bufio.SplitFunc) *readerBuilder {
	b.splitFunc = s
	return b
}

func (b *readerBuilder) withFile(f *os.File) *readerBuilder {
	b.file = f
	return b
}

func (b *readerBuilder) withFingerprint(fp *fingerprint.Fingerprint) *readerBuilder {
	b.fp = fp
	return b
}

func (b *readerBuilder) withOffset(offset int64) *readerBuilder {
	b.offset = offset
	return b
}

func (b *readerBuilder) withHeaderFinalized(finalized bool) *readerBuilder {
	b.headerFinalized = finalized
	return b
}

func (b *readerBuilder) withFileAttributes(attrs map[string]any) *readerBuilder {
	b.fileAttributes = attrs
	return b
}

func (b *readerBuilder) build() (r *Reader, err error) {
	r = &Reader{
		readerConfig:    b.readerConfig,
		Offset:          b.offset,
		headerSettings:  b.headerSettings,
		HeaderFinalized: b.headerFinalized,
		FileAttributes:  b.fileAttributes,
	}

	if b.splitFunc != nil {
		r.lineSplitFunc = b.splitFunc
	} else {
		r.lineSplitFunc, err = b.splitterFactory.Build(b.readerConfig.maxLogSize)
		if err != nil {
			return nil, err
		}
	}

	r.encoding, err = b.encodingConfig.Build()
	if err != nil {
		return nil, err
	}

	if b.headerSettings == nil || b.headerFinalized {
		r.splitFunc = r.lineSplitFunc
		r.processFunc = b.readerConfig.emit
	} else {
		// We are reading the header. Use the header split func
		r.splitFunc = b.headerSettings.splitFunc
		r.processFunc = r.consumeHeaderLine

		// Create the header pipeline
		r.headerPipelineOutput = newHeaderPipelineOutput(b.SugaredLogger)
		r.headerPipeline, err = pipeline.Config{
			Operators:     b.headerSettings.config.MetadataOperators,
			DefaultOutput: r.headerPipelineOutput,
		}.Build(b.SugaredLogger)
		if err != nil {
			return nil, fmt.Errorf("failed to build pipeline: %w", err)
		}
		if err = r.headerPipeline.Start(storage.NewNopClient()); err != nil {
			return nil, fmt.Errorf("failed to start header pipeline: %w", err)
		}
	}

	if b.file == nil {
		r.SugaredLogger = b.SugaredLogger.With("path", "uninitialized")
		return r, nil
	}

	r.file = b.file
	r.SugaredLogger = b.SugaredLogger.With("path", b.file.Name())
	r.FileAttributes = b.fileAttributes

	// Resolve file name and path attributes
	resolved := b.file.Name()

	// Dirty solution, waiting for this permanent fix https://github.com/golang/go/issues/39786
	// EvalSymlinks on windows is partially working depending on the way you use Symlinks and Junctions
	if runtime.GOOS != "windows" {
		resolved, err = filepath.EvalSymlinks(b.file.Name())
		if err != nil {
			b.Errorf("resolve symlinks: %w", err)
		}
	}
	abs, err := filepath.Abs(resolved)
	if err != nil {
		b.Errorf("resolve abs: %w", err)
	}

	if b.readerConfig.includeFileName {
		r.FileAttributes[logFileName] = filepath.Base(b.file.Name())
	} else if r.FileAttributes[logFileName] != nil {
		delete(r.FileAttributes, logFileName)
	}
	if b.readerConfig.includeFilePath {
		r.FileAttributes[logFilePath] = b.file.Name()
	} else if r.FileAttributes[logFilePath] != nil {
		delete(r.FileAttributes, logFilePath)
	}
	if b.readerConfig.includeFileNameResolved {
		r.FileAttributes[logFileNameResolved] = filepath.Base(abs)
	} else if r.FileAttributes[logFileNameResolved] != nil {
		delete(r.FileAttributes, logFileNameResolved)
	}
	if b.readerConfig.includeFilePathResolved {
		r.FileAttributes[logFilePathResolved] = abs
	} else if r.FileAttributes[logFilePathResolved] != nil {
		delete(r.FileAttributes, logFilePathResolved)
	}

	if !b.fromBeginning {
		if err = r.offsetToEnd(); err != nil {
			return nil, err
		}
	}

	if b.fp != nil {
		r.Fingerprint = b.fp
		return r, nil
	}

	fp, err := b.readerFactory.newFingerprint(r.file)
	if err != nil {
		return nil, err
	}
	r.Fingerprint = fp

	return r, nil
}
