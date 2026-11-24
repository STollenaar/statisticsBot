// Package restore rebuilds a DuckDB database from a backup archive produced by
// the backup package.
//
// A backup is not a copy of statsbot.db: it is DuckDB's EXPORT DATABASE output,
// a schema.sql/load.sql pair plus one Parquet file per table, tarred and
// zstd-compressed. That is deliberate — the storage format is not guaranteed
// readable across DuckDB versions while the export is — but it means a restore
// cannot be a file copy either. It has to be an IMPORT DATABASE into a fresh
// file.
//
// Nothing here imports internal/database. That package opens the live database
// in its init and exits the process if it cannot, which is right for the bot and
// wrong for a tool whose whole job is to build a different database beside it.
package restore

import (
	"archive/tar"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"filippo.io/age"
	"github.com/klauspost/compress/zstd"
	_ "github.com/marcboeker/go-duckdb/v2"
)

// DatabaseFile is the name the bot opens inside DUCKDB_PATH.
const DatabaseFile = "statsbot.db"

// Options configures a restore.
type Options struct {
	// Archive is the .tar.zst (or .tar.zst.age) to import.
	Archive string

	// IdentityFile is a file of age private keys used to decrypt an encrypted
	// archive. Empty falls back to BACKUP_IDENTITY_FILE, then to an inline key
	// in BACKUP_IDENTITY. A plaintext archive needs none of them.
	IdentityFile string

	// OutDir is where the rebuilt database is written. A statsbot.db must not
	// already exist there.
	OutDir string

	// StagingDir holds the extracted Parquet files. Defaults to a temporary
	// directory inside OutDir, because the export is roughly the size of the
	// data and the volume with room for the database is the one with room for
	// this too.
	StagingDir string

	// KeepStaging leaves the extracted export in place, for inspecting the
	// Parquet files directly.
	KeepStaging bool
}

// Report describes what a restore produced.
type Report struct {
	DatabasePath string
	Tables       []TableReport
	Extracted    int64
	Took         time.Duration
	// Encrypted records whether the archive had to be decrypted, so a restore
	// from a plaintext backup is distinguishable in the output.
	Encrypted bool
}

// TableReport is one table's row count, checked against the Parquet file it was
// loaded from.
type TableReport struct {
	Name    string
	Rows    int64
	Parquet int64
}

// Mismatch reports whether the imported table disagrees with its source file.
func (t TableReport) Mismatch() bool { return t.Rows != t.Parquet }

// Run extracts the archive and imports it into a new database file.
func Run(opts Options) (Report, error) {
	start := time.Now()

	if opts.Archive == "" {
		return Report{}, errors.New("no archive given")
	}
	if opts.OutDir == "" {
		return Report{}, errors.New("no output directory given")
	}
	if err := os.MkdirAll(opts.OutDir, 0o755); err != nil {
		return Report{}, fmt.Errorf("failed to create %s: %w", opts.OutDir, err)
	}

	target := filepath.Join(opts.OutDir, DatabaseFile)
	// Refused rather than overwritten: IMPORT DATABASE into a database that
	// already has these tables fails halfway through, leaving something that
	// looks like a database and is not one.
	if _, err := os.Stat(target); err == nil {
		return Report{}, fmt.Errorf("%s already exists; move it aside or pick another -out", target)
	} else if !errors.Is(err, os.ErrNotExist) {
		return Report{}, err
	}

	staging := opts.StagingDir
	if staging == "" {
		var err error
		if staging, err = os.MkdirTemp(opts.OutDir, "export-"); err != nil {
			return Report{}, fmt.Errorf("failed to create a staging directory: %w", err)
		}
	} else if err := os.MkdirAll(staging, 0o755); err != nil {
		return Report{}, fmt.Errorf("failed to create %s: %w", staging, err)
	}
	if !opts.KeepStaging {
		defer func() {
			if err := os.RemoveAll(staging); err != nil {
				slog.Error("failed to clean up the staging directory",
					slog.String("dir", staging), slog.Any("err", err))
			}
		}()
	}

	encrypted, err := IsEncrypted(opts.Archive)
	if err != nil {
		return Report{}, err
	}
	var keys []age.Identity
	if encrypted {
		if keys, err = identities(opts.IdentityFile); err != nil {
			return Report{}, err
		}
	}

	slog.Info("extracting archive",
		slog.String("archive", opts.Archive),
		slog.String("into", staging),
		slog.Bool("encrypted", encrypted))
	extracted, err := extract(opts.Archive, staging, keys)
	if err != nil {
		return Report{}, err
	}

	// A truncated or wrong-shaped archive is worth catching before DuckDB tries
	// to run it, because IMPORT DATABASE on a missing load.sql is an unhelpful
	// IO error about a path the caller never typed.
	for _, required := range []string{"schema.sql", "load.sql"} {
		if _, err := os.Stat(filepath.Join(staging, required)); err != nil {
			return Report{}, fmt.Errorf("archive is not a database export: no %s inside", required)
		}
	}

	slog.Info("importing into a new database", slog.String("path", target))
	tables, err := importInto(target, staging)
	if err != nil {
		// Leave nothing half-built behind for a later run to trip over.
		os.Remove(target)
		os.Remove(target + ".wal")
		return Report{}, err
	}

	return Report{
		DatabasePath: target,
		Tables:       tables,
		Extracted:    extracted,
		Took:         time.Since(start),
		Encrypted:    encrypted,
	}, nil
}

// importInto creates a database at target and runs IMPORT DATABASE from dir,
// then counts every table against the Parquet file it came from.
func importInto(target, dir string) ([]TableReport, error) {
	db, err := sql.Open("duckdb", target)
	if err != nil {
		return nil, fmt.Errorf("failed to create %s: %w", target, err)
	}
	defer db.Close()
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to open %s: %w", target, err)
	}

	if _, err := db.Exec(fmt.Sprintf(`IMPORT DATABASE '%s'`, quote(dir))); err != nil {
		return nil, fmt.Errorf("failed to import the export: %w", err)
	}
	// Fold the WAL into the file, so what gets copied or swapped into place is
	// the whole database rather than a file plus a journal.
	if _, err := db.Exec(`CHECKPOINT`); err != nil {
		slog.Warn("could not checkpoint after import", slog.Any("err", err))
	}

	return verify(db, dir)
}

// verify counts each imported table and the Parquet file it was loaded from.
//
// This is the check worth having: load.sql is a series of COPY statements, and a
// restore that quietly loaded nine tables out of eleven would otherwise look
// exactly like a successful one.
func verify(db *sql.DB, dir string) ([]TableReport, error) {
	files, err := filepath.Glob(filepath.Join(dir, "*.parquet"))
	if err != nil {
		return nil, err
	}
	sort.Strings(files)

	var reports []TableReport
	for _, file := range files {
		table := strings.TrimSuffix(filepath.Base(file), ".parquet")

		var inFile int64
		if err := db.QueryRow(
			fmt.Sprintf(`SELECT count(*) FROM read_parquet('%s')`, quote(file)),
		).Scan(&inFile); err != nil {
			return nil, fmt.Errorf("failed to count rows in %s: %w", filepath.Base(file), err)
		}

		var imported int64
		// The table name comes from a filename inside the archive, so it is
		// quoted as an identifier rather than interpolated bare.
		if err := db.QueryRow(
			fmt.Sprintf(`SELECT count(*) FROM "%s"`, strings.ReplaceAll(table, `"`, `""`)),
		).Scan(&imported); err != nil {
			return nil, fmt.Errorf("table %s did not import: %w", table, err)
		}

		reports = append(reports, TableReport{Name: table, Rows: imported, Parquet: inFile})
	}
	if len(reports) == 0 {
		return nil, errors.New("the archive contained no table data")
	}
	return reports, nil
}

// Replace swaps a restored database into place, keeping what was there.
//
// The old database and its write-ahead log are renamed rather than deleted, and
// the WAL has to move with it: DuckDB replays a stale .wal against whatever .db
// it finds beside it, which fails with an internal error about replaying a log
// that does not match. A rename also means a bot still holding the old file
// keeps serving from it unharmed — it simply will not see the restored data
// until it restarts.
func Replace(restored, duckdbPath string) (backup string, err error) {
	target := filepath.Join(duckdbPath, DatabaseFile)
	stamp := time.Now().UTC().Format("20060102T150405Z")

	if _, err := os.Stat(target); err == nil {
		backup = fmt.Sprintf("%s.%s.bak", target, stamp)
		if err := os.Rename(target, backup); err != nil {
			return "", fmt.Errorf("failed to move the existing database aside: %w", err)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", err
	}

	if _, err := os.Stat(target + ".wal"); err == nil {
		if err := os.Rename(target+".wal", fmt.Sprintf("%s.wal.%s.bak", target, stamp)); err != nil {
			return backup, fmt.Errorf("failed to move the existing write-ahead log aside: %w", err)
		}
	}

	if err := os.Rename(restored, target); err != nil {
		return backup, fmt.Errorf("failed to move the restored database into place: %w", err)
	}
	return backup, nil
}

// extract unpacks a tar.zst into dir, returning the number of bytes written.
// When keys are given the archive is decrypted on the way through, undoing the
// tar -> zstd -> age layering the backup applied.
func extract(archive, dir string, keys []age.Identity) (int64, error) {
	file, err := os.Open(archive)
	if err != nil {
		return 0, fmt.Errorf("failed to open %s: %w", archive, err)
	}
	defer file.Close()

	var source io.Reader = file
	if len(keys) > 0 {
		if source, err = age.Decrypt(file, keys...); err != nil {
			// age reports a wrong key as "no identity matched any of the
			// recipients", which is the common mistake and worth not dressing
			// up: the archive is fine, the key is the wrong one.
			return 0, fmt.Errorf("failed to decrypt %s: %w", archive, err)
		}
	}

	decoder, err := zstd.NewReader(source)
	if err != nil {
		return 0, fmt.Errorf("failed to read %s as zstd: %w", archive, err)
	}
	defer decoder.Close()

	var written int64
	reader := tar.NewReader(decoder)
	for {
		header, err := reader.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return written, fmt.Errorf("failed to read the archive: %w", err)
		}

		// The backup writes nothing but regular files, so anything else is
		// either a different kind of archive or something trying to plant a
		// symlink outside the staging directory.
		if header.Typeflag != tar.TypeReg {
			slog.Warn("skipping a non-regular archive entry",
				slog.String("name", header.Name), slog.Any("type", header.Typeflag))
			continue
		}

		target, err := safeJoin(dir, header.Name)
		if err != nil {
			return written, err
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return written, err
		}

		n, err := writeFile(target, reader, header.FileInfo().Mode().Perm())
		written += n
		if err != nil {
			return written, err
		}
	}
	return written, nil
}

func writeFile(target string, r io.Reader, mode os.FileMode) (int64, error) {
	out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return 0, fmt.Errorf("failed to create %s: %w", target, err)
	}
	defer out.Close()

	n, err := io.Copy(out, r)
	if err != nil {
		return n, fmt.Errorf("failed to write %s: %w", target, err)
	}
	return n, nil
}

// safeJoin resolves an archive entry's name inside dir, rejecting anything that
// would land outside it.
//
// The archive comes from a bucket rather than from the caller, so an entry named
// ../../etc/something is worth refusing even though the backup job would never
// write one.
func safeJoin(dir, name string) (string, error) {
	if name == "" {
		return "", errors.New("archive contains an entry with no name")
	}
	if filepath.IsAbs(name) || strings.HasPrefix(name, "/") {
		return "", fmt.Errorf("archive entry %q is an absolute path", name)
	}
	target := filepath.Join(dir, filepath.FromSlash(name))
	if target != dir && !strings.HasPrefix(target, filepath.Clean(dir)+string(os.PathSeparator)) {
		return "", fmt.Errorf("archive entry %q escapes the staging directory", name)
	}
	return target, nil
}

// quote escapes a value for a single-quoted DuckDB string literal. Paths cannot
// be bound as parameters in IMPORT DATABASE or read_parquet.
func quote(s string) string {
	return strings.ReplaceAll(s, "'", "''")
}

// ageIntro is the first line of every age file, in the binary format the backup
// writes. Detecting it means an archive carries whether it is encrypted, so the
// restore does not depend on the filename still ending in .age after passing
// through a browser's downloads folder.
const ageIntro = "age-encryption.org/v1\n"

// IsEncrypted reports whether an archive is age-encrypted, by its contents
// rather than its name.
func IsEncrypted(archive string) (bool, error) {
	file, err := os.Open(archive)
	if err != nil {
		return false, fmt.Errorf("failed to open %s: %w", archive, err)
	}
	defer file.Close()

	intro := make([]byte, len(ageIntro))
	n, err := io.ReadFull(file, intro)
	// A file too short to hold the intro cannot be encrypted; whether it is a
	// usable archive at all is settled later, with a better error.
	if errors.Is(err, io.ErrUnexpectedEOF) || errors.Is(err, io.EOF) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("failed to read %s: %w", archive, err)
	}
	return string(intro[:n]) == ageIntro, nil
}

// identities loads the private keys for decrypting a backup.
//
// Checked in order: the path given on the command line, BACKUP_IDENTITY_FILE,
// then BACKUP_IDENTITY holding the key itself. The inline form is there for a
// one-off restore in a shell, and is the worse option — it puts a private key
// in the process environment, where it is visible to anything that can read
// /proc and tends to end up in shell history.
func identities(identityFile string) ([]age.Identity, error) {
	if identityFile == "" {
		identityFile = os.Getenv("BACKUP_IDENTITY_FILE")
	}

	if identityFile != "" {
		file, err := os.Open(identityFile)
		if err != nil {
			return nil, fmt.Errorf("failed to open the identity file: %w", err)
		}
		defer file.Close()

		keys, err := age.ParseIdentities(file)
		if err != nil {
			return nil, fmt.Errorf("failed to read %s as age identities: %w", identityFile, err)
		}
		return keys, nil
	}

	if inline := os.Getenv("BACKUP_IDENTITY"); inline != "" {
		keys, err := age.ParseIdentities(strings.NewReader(inline))
		if err != nil {
			return nil, fmt.Errorf("failed to read BACKUP_IDENTITY as an age identity: %w", err)
		}
		return keys, nil
	}

	return nil, errors.New("this archive is encrypted: pass -identity <file>, " +
		"or set BACKUP_IDENTITY_FILE, with the age private key it was encrypted to")
}
