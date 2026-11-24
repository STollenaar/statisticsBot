// Command backupkey generates the key pair that backups are encrypted to.
//
//	go run ./tools/backupkey > backup-identity.key
//
// It prints the private key to stdout and the public recipient to stderr, so
// redirecting stdout captures the secret and leaves the part you need to paste
// into the deployment visible in the terminal.
//
// The public key goes in BACKUP_RECIPIENTS, where the bot uses it to encrypt and
// nothing else. The private key is the only thing that can read those backups:
// keep it off the cluster and somewhere it will outlive the machine it was
// generated on, because losing it makes every backup encrypted to it unreadable.
// There is no recovery path, by design.
package main

import (
	"fmt"
	"os"

	"filippo.io/age"
)

func main() {
	identity, err := age.GenerateX25519Identity()
	if err != nil {
		fmt.Fprintf(os.Stderr, "backupkey: %v\n", err)
		os.Exit(1)
	}

	// age's own comment convention, so the file is readable by the age CLI as
	// well as by tools/restore.
	fmt.Printf("# public key: %s\n%s\n", identity.Recipient(), identity)

	fmt.Fprintf(os.Stderr, `
Generated an age key pair.

  Set on the deployment (public, not a secret):
    BACKUP_RECIPIENTS=%s

  The private key went to stdout. Store it offline; it is the only way to read
  backups encrypted to it, and it is not recoverable.

  Restore with:
    go run ./tools/restore -archive <file>.tar.zst.age -identity <this file>
`, identity.Recipient())
}
