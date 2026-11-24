// Command restore rebuilds statsbot.db from a backup archive.
//
//	go run ./tools/restore -archive statsbot-2026-11-16T03-00-04Z.tar.zst
//
// That writes ./restored/statsbot.db and reports every table's row count against
// the Parquet file it came from. To put it into service, either copy that file
// over the one the bot opens, or point the tool at the live directory:
//
//	go run ./tools/restore -archive <file> -replace /duckdb
//
// -replace renames the current database and its write-ahead log aside with a
// timestamp rather than deleting them, so the previous state is recoverable.
// Stop the bot first: a rename will not corrupt a running one, but it will keep
// serving the old data from the file it already has open until it restarts.
//
// An encrypted archive is decrypted on the way through; pass -identity with the
// age private key, or set BACKUP_IDENTITY_FILE. Whether an archive is encrypted
// is read from its contents, not its name, so a download that lost the .age
// suffix still works.
//
// Budget disk for roughly three times the archive size: the extracted export,
// the database built from it, and the archive itself.
package main

import (
	"flag"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"text/tabwriter"

	"github.com/stollenaar/statisticsbot/internal/restore"
)

func main() {
	archive := flag.String("archive", "", "backup .tar.zst to import (required)")
	out := flag.String("out", "restored", "directory to write the rebuilt statsbot.db into")
	replace := flag.String("replace", "", "after a successful import, swap the result into this DUCKDB_PATH")
	keepStaging := flag.Bool("keep-staging", false, "leave the extracted parquet export in place")
	identity := flag.String("identity", "", "age private key file for an encrypted archive (default $BACKUP_IDENTITY_FILE)")
	flag.Parse()

	if *archive == "" {
		fmt.Fprintln(os.Stderr, "restore: -archive is required")
		flag.Usage()
		os.Exit(2)
	}

	report, err := restore.Run(restore.Options{
		Archive:      *archive,
		OutDir:       *out,
		KeepStaging:  *keepStaging,
		IdentityFile: *identity,
	})
	if err != nil {
		slog.Error("restore failed", slog.Any("err", err))
		os.Exit(1)
	}

	writer := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(writer, "TABLE\tROWS\tIN ARCHIVE")
	var mismatched bool
	for _, table := range report.Tables {
		note := ""
		if table.Mismatch() {
			note, mismatched = "\tMISMATCH", true
		}
		fmt.Fprintf(writer, "%s\t%d\t%d%s\n", table.Name, table.Rows, table.Parquet, note)
	}
	writer.Flush()

	encryption := "plaintext archive"
	if report.Encrypted {
		encryption = "decrypted"
	}
	fmt.Printf("\nrestored %s (%d tables, %.1f GiB extracted, %s) in %s\n",
		report.DatabasePath, len(report.Tables),
		float64(report.Extracted)/(1<<30), encryption, report.Took.Round(1e9))

	// A row-count mismatch means load.sql did not fully apply. Stopping here
	// keeps a half-loaded database from being swapped into place by -replace.
	if mismatched {
		slog.Error("imported row counts do not match the archive; not swapping anything into place")
		os.Exit(1)
	}

	if *replace == "" {
		fmt.Printf("\nto put it into service:\n  cp %s %s\n",
			report.DatabasePath, filepath.Join("<DUCKDB_PATH>", restore.DatabaseFile))
		return
	}

	backup, err := restore.Replace(report.DatabasePath, *replace)
	if err != nil {
		slog.Error("failed to swap the restored database into place", slog.Any("err", err))
		os.Exit(1)
	}
	if backup != "" {
		fmt.Printf("\nprevious database kept at %s\n", backup)
	}
	fmt.Printf("restored into %s; restart the bot to pick it up\n",
		filepath.Join(*replace, restore.DatabaseFile))
}
