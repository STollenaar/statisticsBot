package backup

import (
	"archive/tar"
	"bytes"
	"crypto/rand"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	"filippo.io/age"
	"github.com/klauspost/compress/zstd"

	"github.com/stollenaar/statisticsbot/internal/util"
)

// These tests need a writable DUCKDB_PATH: this package imports
// internal/database for ExportSnapshot, and that package opens a database in its
// init.
//
// Give each package its own DUCKDB_PATH. `go test ./...` with one shared value
// fails: DuckDB allows a single writer, and the test binaries run in parallel.
//
//	DUCKDB_PATH=$(mktemp -d) go test ./internal/backup/
//	DUCKDB_PATH=$(mktemp -d) go test ./internal/util/charts/
//
// STATSBOT_EXPORT_DIR points the round trip at a real EXPORT DATABASE directory
// instead of the synthetic one, which is how the production-sized archive was
// checked end to end:
//
//	go run ./tools/restore -archive <backup>.tar.zst -out /tmp/x -keep-staging
//	STATSBOT_EXPORT_DIR=/tmp/x/export-* DUCKDB_PATH=/tmp go test ./internal/backup/ -run RoundTrip

// exportDir builds something shaped like a database export: the two SQL files
// plus a payload comfortably larger than age's 64 KiB chunk, so the STREAM
// framing is exercised across chunks rather than fitting in one.
func exportDir(t *testing.T) (dir string, contents map[string][]byte) {
	t.Helper()
	if real := os.Getenv("STATSBOT_EXPORT_DIR"); real != "" {
		return real, nil
	}

	dir = t.TempDir()
	contents = map[string][]byte{
		"schema.sql": []byte("CREATE TABLE messages(id VARCHAR);\n"),
		"load.sql":   []byte("COPY messages FROM 'messages.parquet';\n"),
	}
	payload := make([]byte, 700*1024)
	if _, err := rand.Read(payload); err != nil {
		t.Fatal(err)
	}
	contents["messages.parquet"] = payload

	for name, data := range contents {
		if err := os.WriteFile(filepath.Join(dir, name), data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir, contents
}

// readArchive reverses writeArchive: decrypt, decompress, untar.
func readArchive(t *testing.T, archive []byte, keys ...age.Identity) map[string][]byte {
	t.Helper()
	var source io.Reader = bytes.NewReader(archive)
	if len(keys) > 0 {
		decrypted, err := age.Decrypt(source, keys...)
		if err != nil {
			t.Fatalf("decrypting: %v", err)
		}
		source = decrypted
	}
	decoder, err := zstd.NewReader(source)
	if err != nil {
		t.Fatalf("decompressing: %v", err)
	}
	defer decoder.Close()

	out := map[string][]byte{}
	reader := tar.NewReader(decoder)
	for {
		header, err := reader.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatalf("reading the archive: %v", err)
		}
		data, err := io.ReadAll(reader)
		if err != nil {
			t.Fatalf("reading %s: %v", header.Name, err)
		}
		out[header.Name] = data
	}
	return out
}

// STATSBOT_ARCHIVE_OUT writes the encrypted archive to a file and prints the
// key, so the production-sized archive can be handed to the real restore tool
// instead of being held in memory twice:
//
//	STATSBOT_EXPORT_DIR=... STATSBOT_ARCHIVE_OUT=/tmp/enc.tar.zst.age \
//	  DUCKDB_PATH=/tmp go test ./internal/backup/ -run ToFile -v
func TestRealExportToFile(t *testing.T) {
	out := os.Getenv("STATSBOT_ARCHIVE_OUT")
	if out == "" || os.Getenv("STATSBOT_EXPORT_DIR") == "" {
		t.Skip("set STATSBOT_EXPORT_DIR and STATSBOT_ARCHIVE_OUT")
	}
	dir, _ := exportDir(t)

	identity, err := age.GenerateX25519Identity()
	if err != nil {
		t.Fatal(err)
	}
	file, err := os.Create(out)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()

	if err := writeArchive(file, dir, []age.Recipient{identity.Recipient()}); err != nil {
		t.Fatalf("writeArchive: %v", err)
	}
	info, err := file.Stat()
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("wrote %s (%.2f GiB)", out, float64(info.Size())/(1<<30))
	t.Logf("identity: %s", identity)
	t.Logf("recipient: %s", identity.Recipient())
}

// An encrypted archive must come back byte-for-byte, and must not be readable
// without the key.
func TestWriteArchiveRoundTrip(t *testing.T) {
	dir, want := exportDir(t)

	identity, err := age.GenerateX25519Identity()
	if err != nil {
		t.Fatal(err)
	}

	var encrypted bytes.Buffer
	if err := writeArchive(&encrypted, dir, []age.Recipient{identity.Recipient()}); err != nil {
		t.Fatalf("writeArchive: %v", err)
	}

	// The ciphertext must not contain the plaintext it was built from. This
	// would catch the layers being wired in the wrong order, or the payload
	// bypassing the encrypting writer entirely.
	if bytes.Contains(encrypted.Bytes(), []byte("CREATE TABLE messages")) {
		t.Error("plaintext is visible in the encrypted archive")
	}
	if !bytes.HasPrefix(encrypted.Bytes(), []byte(ageIntroForTest)) {
		t.Errorf("archive does not start with the age header: %q", encrypted.Bytes()[:32])
	}

	got := readArchive(t, encrypted.Bytes(), identity)
	if want == nil {
		// Pointed at a real export: assert it round-tripped something
		// substantial rather than checking contents we did not write.
		if len(got) < 3 {
			t.Fatalf("round-tripped only %d files", len(got))
		}
		var total int
		for _, data := range got {
			total += len(data)
		}
		t.Logf("round-tripped %d files, %.1f MiB, from %.1f MiB of ciphertext",
			len(got), float64(total)/(1<<20), float64(encrypted.Len())/(1<<20))
		return
	}

	if len(got) != len(want) {
		t.Fatalf("got %d files back, want %d", len(got), len(want))
	}
	for name, data := range want {
		if !bytes.Equal(got[name], data) {
			t.Errorf("%s came back different (%d bytes vs %d)", name, len(got[name]), len(data))
		}
	}
}

// The wrong key must fail rather than produce something.
func TestWrongKeyCannotDecrypt(t *testing.T) {
	dir, _ := exportDir(t)

	mine, err := age.GenerateX25519Identity()
	if err != nil {
		t.Fatal(err)
	}
	theirs, err := age.GenerateX25519Identity()
	if err != nil {
		t.Fatal(err)
	}

	var encrypted bytes.Buffer
	if err := writeArchive(&encrypted, dir, []age.Recipient{mine.Recipient()}); err != nil {
		t.Fatal(err)
	}
	if _, err := age.Decrypt(bytes.NewReader(encrypted.Bytes()), theirs); err == nil {
		t.Error("an unrelated key decrypted the archive")
	}
}

// Listing several recipients must let any one of them decrypt, which is what
// makes rotating a key possible without re-encrypting history.
func TestMultipleRecipients(t *testing.T) {
	dir, want := exportDir(t)
	if want == nil {
		t.Skip("pointed at a real export")
	}

	first, err := age.GenerateX25519Identity()
	if err != nil {
		t.Fatal(err)
	}
	second, err := age.GenerateX25519Identity()
	if err != nil {
		t.Fatal(err)
	}

	var encrypted bytes.Buffer
	if err := writeArchive(&encrypted, dir,
		[]age.Recipient{first.Recipient(), second.Recipient()}); err != nil {
		t.Fatal(err)
	}
	for name, key := range map[string]age.Identity{"first": first, "second": second} {
		if _, err := age.Decrypt(bytes.NewReader(encrypted.Bytes()), key); err != nil {
			t.Errorf("%s recipient could not decrypt: %v", name, err)
		}
	}
}

// Truncation must be detected. A backup that silently decrypts to a prefix is
// worse than one that fails, because it restores into a database that looks
// complete.
func TestTruncationIsDetected(t *testing.T) {
	dir, want := exportDir(t)
	if want == nil {
		t.Skip("pointed at a real export")
	}

	identity, err := age.GenerateX25519Identity()
	if err != nil {
		t.Fatal(err)
	}
	var encrypted bytes.Buffer
	if err := writeArchive(&encrypted, dir, []age.Recipient{identity.Recipient()}); err != nil {
		t.Fatal(err)
	}

	cut := encrypted.Bytes()[:encrypted.Len()-1024]
	reader, err := age.Decrypt(bytes.NewReader(cut), identity)
	if err != nil {
		return // refused at the header, which is also a detection
	}
	if _, err := io.ReadAll(reader); err == nil {
		t.Error("a truncated archive decrypted without complaint")
	}
}

// Encryption has to be what happens by default; plaintext has to be asked for.
func TestRecipientsFailClosed(t *testing.T) {
	original := util.ConfigFile
	t.Cleanup(func() { util.ConfigFile = original })

	cases := []struct {
		name       string
		recipients string
		encryption string
		want       int
		wantErr    error
	}{
		{name: "unset refuses", wantErr: ErrNoRecipients},
		{name: "explicitly off allows plaintext", encryption: "off", want: 0},
		{name: "off is case insensitive", encryption: "OFF", want: 0},
		{name: "a typo still refuses", encryption: "false", wantErr: ErrNoRecipients},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			util.ConfigFile = &util.Config{
				BACKUP_RECIPIENTS: tc.recipients,
				BACKUP_ENCRYPTION: tc.encryption,
			}
			got, err := recipients()
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("got error %v, want %v", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(got) != tc.want {
				t.Fatalf("got %d recipients, want %d", len(got), tc.want)
			}
		})
	}

	// A literal key short-circuits the parameter lookup. Worth pinning: the
	// getter must not reach for SSM when the key is already in hand, both
	// because it would be a pointless call per backup and because it would make
	// this test depend on AWS.
	util.ConfigFile = &util.Config{
		BACKUP_RECIPIENTS:     "age1vhgtw7uc2y3hv92snfwrlvanpmk7mk922rtsk0qtuk44gpx0fsmsaagvs8",
		AWS_BACKUP_RECIPIENTS: "/statisticsbot/encryption_public",
	}
	if got, err := recipients(); err != nil || len(got) != 1 {
		t.Fatalf("a literal recipient should win over the parameter path: %d, %v", len(got), err)
	}

	// A configured recipient wins regardless of the opt-out, and the separators
	// a manifest is likely to use all work.
	identity, err := age.GenerateX25519Identity()
	if err != nil {
		t.Fatal(err)
	}
	other, err := age.GenerateX25519Identity()
	if err != nil {
		t.Fatal(err)
	}
	for _, separator := range []string{",", " ", ", ", "\n", ";"} {
		util.ConfigFile = &util.Config{
			BACKUP_RECIPIENTS: identity.Recipient().String() + separator + other.Recipient().String(),
		}
		got, err := recipients()
		if err != nil {
			t.Fatalf("separator %q: %v", separator, err)
		}
		if len(got) != 2 {
			t.Errorf("separator %q parsed %d recipients, want 2", separator, len(got))
		}
	}

	// Garbage must be reported, not ignored into an unencrypted upload.
	util.ConfigFile = &util.Config{BACKUP_RECIPIENTS: "not-a-key"}
	if _, err := recipients(); err == nil {
		t.Error("an unparseable recipient was accepted")
	}
}

// ageIntroForTest mirrors the constant in internal/restore; duplicated rather
// than exported, so the two packages agree on the format independently.
const ageIntroForTest = "age-encryption.org/v1\n"
