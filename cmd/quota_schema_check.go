package cmd

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"
	_ "modernc.org/sqlite"
)

// quotaSchemaCheck runs only against a new private fixture directory. It reads
// the SQLite identity and exercises the supplied candidate schema in this build.
func init() {
	rootCmd.AddCommand(&cobra.Command{
		Use: "quota-schema-check SCHEMA.sql PRIVATE-NEW-DIRECTORY", Hidden: true,
		Args:              cobra.ExactArgs(2),
		PersistentPreRunE: func(*cobra.Command, []string) error { return nil },
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := os.Mkdir(args[1], 0700); err != nil {
				return err
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), 20*time.Second)
			defer cancel()
			return checkQuotaSchema(ctx, cmd, args[0], args[1])
		},
	})
}
func checkQuotaSchema(ctx context.Context, cmd *cobra.Command, schema, dir string) error {
	data, err := os.ReadFile(schema)
	if err != nil {
		return err
	}
	// Only a trusted copy of the packaged DDL is accepted here.
	sum := fmt.Sprintf("%x", sha256.Sum256(data))
	if sum != "44eabbffe05e689ca9893be752b1f7d07305e938b13a8078dae606e57946bd8f" {
		return fmt.Errorf("unexpected schema hash")
	}
	db, err := openLabDB(dir)
	if err != nil {
		return err
	}
	defer db.Close()
	if _, err = db.ExecContext(ctx, string(data)); err != nil {
		return err
	}
	var version, source, journal string
	var syncMode, foreignKeys, wait, schemaVersion int
	reads := []struct {
		query string
		value any
	}{
		{"SELECT sqlite_version()", &version}, {"SELECT sqlite_source_id()", &source},
		{"PRAGMA journal_mode", &journal}, {"PRAGMA synchronous", &syncMode},
		{"PRAGMA foreign_keys", &foreignKeys}, {"PRAGMA busy_timeout", &wait}, {"PRAGMA user_version", &schemaVersion},
	}
	for _, r := range reads {
		if err = db.QueryRowContext(ctx, r.query).Scan(r.value); err != nil {
			return err
		}
	}
	if journal != "wal" || syncMode != 2 || foreignKeys != 1 || wait != 250 || schemaVersion != 1 {
		return fmt.Errorf("unexpected effective SQLite settings")
	}
	return json.NewEncoder(cmd.OutOrStdout()).Encode(map[string]any{"sqlite_version": version, "sqlite_source_id": source, "journal_mode": journal, "synchronous": syncMode, "foreign_keys": foreignKeys, "busy_timeout_ms": wait, "schema_version": schemaVersion, "schema_sha256": sum})
}
func openLabDB(dir string) (*sql.DB, error) {
	path, err := filepath.Abs(filepath.Join(dir, "ledger.sqlite"))
	if err != nil {
		return nil, err
	}
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return nil, err
	}
	if err = f.Close(); err != nil {
		return nil, err
	}
	address := url.URL{Scheme: "file", Path: path}
	values := url.Values{"mode": {"rw"}, "_txlock": {"immediate"}, "_pragma": {"journal_mode(WAL)", "synchronous(FULL)", "foreign_keys(ON)", "trusted_schema(OFF)", "busy_timeout(250)"}}
	address.RawQuery = values.Encode()
	db, err := sql.Open("sqlite", address.String())
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	return db, nil
}
