package backup

import (
	"errors"
	"fmt"
	"strings"

	"filippo.io/age"

	"github.com/stollenaar/statisticsbot/internal/util"
)

// A backup is every message the bot has ever seen, so it is encrypted before it
// leaves the process rather than relying on the bucket.
//
// age is used instead of assembling something from crypto/cipher. The archive is
// gigabytes and streams straight into the uploader, which rules out encrypting a
// buffer in one shot: a streaming AEAD has to split the plaintext into chunks,
// number them so they cannot be reordered or dropped, and mark the last one so a
// truncated file fails to decrypt instead of decrypting to a prefix. That is the
// STREAM construction, and age implements exactly it over ChaCha20-Poly1305.
// Writing that framing here is the kind of code that looks right and is not.
//
// The recipients are public keys; see util.Config.BACKUP_RECIPIENTS for why that
// matters. Decryption happens in internal/restore, with a private key the
// cluster never holds.

// AgeFileExtension is appended to an encrypted archive's key, so which backups
// in the bucket are encrypted is visible without downloading one.
const AgeFileExtension = ".age"

// ErrNoRecipients is returned when neither a recipient nor an explicit opt-out
// was configured.
var ErrNoRecipients = errors.New(
	"refusing to upload an unencrypted backup: set BACKUP_RECIPIENTS to one or more " +
		"age public keys, or AWS_BACKUP_RECIPIENTS to the SSM parameter holding them " +
		"(see tools/backupkey), or BACKUP_ENCRYPTION=off to accept plaintext")

// encryptionDisabled reports whether plaintext uploads were explicitly asked
// for. Anything other than "off" leaves encryption on, so a typo fails closed.
func encryptionDisabled() bool {
	return strings.EqualFold(strings.TrimSpace(util.ConfigFile.BACKUP_ENCRYPTION), "off")
}

// recipients resolves the configured age public keys.
//
// Returns nil with no error only when encryption was explicitly disabled; a
// missing configuration is an error, because the alternative is quietly
// uploading the whole message history in the clear.
func recipients() ([]age.Recipient, error) {
	// Read through the getter so AWS_BACKUP_RECIPIENTS resolves from Parameter
	// Store. A lookup that fails is reported rather than treated as "none
	// configured", which would otherwise turn an SSM outage into the
	// indistinguishable complaint that nobody set a recipient.
	keys, err := util.GetBackupRecipients()
	if err != nil {
		return nil, fmt.Errorf("failed to read the backup recipients: %w", err)
	}

	configured := strings.FieldsFunc(keys, func(r rune) bool {
		return r == ',' || r == ';' || r == ' ' || r == '\t' || r == '\n' || r == '\r'
	})

	if len(configured) == 0 {
		if encryptionDisabled() {
			return nil, nil
		}
		return nil, ErrNoRecipients
	}

	var parsed []age.Recipient
	for _, key := range configured {
		recipient, err := age.ParseX25519Recipient(key)
		if err != nil {
			// The key itself is not echoed: it is public, but quoting
			// half-parsed secrets into logs is a habit worth not having, and
			// the position is enough to find it in the manifest.
			return nil, fmt.Errorf("BACKUP_RECIPIENTS entry %d is not an age public key: %w", len(parsed)+1, err)
		}
		parsed = append(parsed, recipient)
	}
	return parsed, nil
}
