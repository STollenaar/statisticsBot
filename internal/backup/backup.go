// Package backup snapshots the DuckDB database and stores the result in a
// Backblaze B2 bucket over its S3-compatible API.
package backup

import (
	"archive/tar"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/feature/s3/transfermanager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/klauspost/compress/zstd"

	"github.com/stollenaar/statisticsbot/internal/database"
	"github.com/stollenaar/statisticsbot/internal/util"
)

const (
	// The archive is streamed from a pipe, so the uploader cannot seek back over
	// it and buffers partSize*concurrency bytes in memory instead.
	partSize    = 8 * 1024 * 1024
	concurrency = 3
)

// ErrInProgress is returned when a backup is already running.
var ErrInProgress = errors.New("a backup is already in progress")

// running serialises backups so an overlapping CronJob retry cannot export into
// a second staging directory while the first one is still on the disk.
var running sync.Mutex

// Result describes a completed backup.
type Result struct {
	Key   string
	Bytes int64
	Took  time.Duration
}

// Run exports the database, packs it into a tar.zst and uploads it to B2 under
// a timestamped key. Nothing is ever overwritten; pruning old backups is left
// to a bucket lifecycle rule so that a bug here cannot delete history.
func Run(ctx context.Context) (Result, error) {
	if !running.TryLock() {
		return Result{}, ErrInProgress
	}
	defer running.Unlock()

	start := time.Now()

	uploader, err := newUploader(ctx)
	if err != nil {
		return Result{}, err
	}

	// Stage the export next to the database: it is roughly the size of the data
	// and the PVC is the only volume with room for it.
	dir, err := os.MkdirTemp(util.ConfigFile.DUCKDB_PATH, "backup-")
	if err != nil {
		return Result{}, fmt.Errorf("failed to create staging directory: %w", err)
	}
	defer func() {
		if err := os.RemoveAll(dir); err != nil {
			slog.Error("failed to clean up staging directory", slog.String("dir", dir), slog.Any("err", err))
		}
	}()

	if err := database.ExportSnapshot(dir); err != nil {
		return Result{}, err
	}

	key := fmt.Sprintf("%s/statsbot-%s.tar.zst", util.ConfigFile.B2_PREFIX, start.UTC().Format("2006-01-02T15-04-05Z"))

	var written atomic.Int64
	reader, writer := io.Pipe()
	go func() {
		// Closing with the archive error surfaces it on the uploader's read side,
		// aborting the upload instead of storing a truncated object.
		writer.CloseWithError(writeArchive(&countingWriter{w: writer, n: &written}, dir))
	}()

	_, err = uploader.UploadObject(ctx, &transfermanager.UploadObjectInput{
		Bucket:      aws.String(util.ConfigFile.B2_BUCKET),
		Key:         aws.String(key),
		Body:        reader,
		ContentType: aws.String("application/zstd"),
	})
	if err != nil {
		reader.CloseWithError(err)
		return Result{}, fmt.Errorf("failed to upload %s: %w", key, err)
	}

	result := Result{Key: key, Bytes: written.Load(), Took: time.Since(start)}
	slog.Info("backup uploaded",
		slog.String("bucket", util.ConfigFile.B2_BUCKET),
		slog.String("key", result.Key),
		slog.Int64("bytes", result.Bytes),
		slog.Duration("took", result.Took.Round(time.Millisecond)),
	)
	return result, nil
}

// newUploader builds an S3 uploader pointed at Backblaze. Credentials are read
// per run so rotating the key in SSM takes effect without a restart.
func newUploader(ctx context.Context) (*transfermanager.Client, error) {
	if util.ConfigFile.B2_BUCKET == "" {
		return nil, errors.New("B2_BUCKET is not set")
	}
	if util.ConfigFile.B2_REGION == "" {
		return nil, errors.New("B2_REGION is not set")
	}
	if util.ConfigFile.B2_ENDPOINT == "" {
		return nil, errors.New("B2_ENDPOINT is not set")
	}

	keyID, err := util.GetB2KeyID()
	if err != nil {
		return nil, err
	}
	applicationKey, err := util.GetB2ApplicationKey()
	if err != nil {
		return nil, err
	}

	// Built by hand rather than through config.LoadDefaultConfig so the Vault
	// injected AWS credentials and AWS_REGION in the pod cannot leak into the
	// Backblaze client.
	cfg := aws.Config{
		Region:      util.ConfigFile.B2_REGION,
		Credentials: credentials.NewStaticCredentialsProvider(keyID, applicationKey, ""),
		// Backblaze rejects the SDK default of a CRC32 trailer sent with
		// Content-Encoding: aws-chunked, so only send checksums where the
		// operation actually requires one.
		RequestChecksumCalculation: aws.RequestChecksumCalculationWhenRequired,
		ResponseChecksumValidation: aws.ResponseChecksumValidationWhenRequired,
	}

	client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(util.ConfigFile.B2_ENDPOINT)
	})

	return transfermanager.New(client, func(o *transfermanager.Options) {
		o.PartSizeBytes = partSize
		o.Concurrency = concurrency
	}), nil
}

// writeArchive tars every file the export produced into w, compressed with
// zstd. The parquet files are already compressed; this mostly pays off on the
// schema.sql/load.sql pair and the tar padding.
func writeArchive(w io.Writer, dir string) error {
	encoder, err := zstd.NewWriter(w, zstd.WithEncoderLevel(zstd.SpeedFastest))
	if err != nil {
		return fmt.Errorf("failed to create zstd writer: %w", err)
	}
	archive := tar.NewWriter(encoder)

	err = filepath.WalkDir(dir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}

		info, err := entry.Info()
		if err != nil {
			return err
		}
		header, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		header.Name = filepath.ToSlash(relative)

		if err := archive.WriteHeader(header); err != nil {
			return err
		}

		file, err := os.Open(path)
		if err != nil {
			return err
		}
		defer file.Close()

		_, err = io.Copy(archive, file)
		return err
	})
	if err != nil {
		return fmt.Errorf("failed to archive export: %w", err)
	}

	if err := archive.Close(); err != nil {
		return fmt.Errorf("failed to finish archive: %w", err)
	}
	return encoder.Close()
}

// countingWriter tracks how many compressed bytes end up in the upload.
type countingWriter struct {
	w io.Writer
	n *atomic.Int64
}

func (c *countingWriter) Write(p []byte) (int, error) {
	written, err := c.w.Write(p)
	c.n.Add(int64(written))
	return written, err
}
