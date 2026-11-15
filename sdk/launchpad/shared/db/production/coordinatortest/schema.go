package coordinatortest

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/db/production"
	"github.com/jackc/pgx/v5"
)

// ReadMigrationSource resolves reviewed monorepo paths, including the original
// service paths retained in migration manifests, and checks their exact bytes.
func ReadMigrationSource(root, relative, want string) ([]byte, error) {
	if filepath.IsAbs(relative) || !filepath.IsLocal(relative) {
		return nil, fmt.Errorf("migration source must be relative to its repository")
	}
	if strings.HasPrefix(relative, "spec/production-system/") {
		relative = "launchpad/" + relative
	}
	for _, prefix := range []string{"core/", "media/"} {
		if strings.HasPrefix(relative, prefix) {
			relative = "services/launchpad/" + strings.TrimPrefix(relative, prefix)
			break
		}
	}
	raw, err := os.ReadFile(filepath.Join(root, relative))
	if err != nil {
		return nil, fmt.Errorf("migration source is unavailable")
	}
	if want != "" && production.Checksum(raw) != want {
		return nil, fmt.Errorf("migration source checksum mismatch")
	}
	return raw, nil
}

// GrantPresentOwnerInterfaces grants reviewed owner reads whose source relations
// exist. Call it before each migration and again after the complete plan.
func GrantPresentOwnerInterfaces(ctx context.Context, tx pgx.Tx, manifests []production.RoleManifest, owner string) error {
	for _, m := range manifests {
		if owner != "" && m.OwnerRole != owner {
			continue
		}
		for _, grant := range m.OwnerReadGrants {
			var present bool
			if err := tx.QueryRow(ctx, "SELECT to_regclass($1) IS NOT NULL", pgx.Identifier{grant.Schema, grant.Object}.Sanitize()).Scan(&present); err != nil {
				return err
			}
			if !present {
				continue
			}
			scoped := m
			scoped.Grants, scoped.SchemaCapabilities = nil, nil
			scoped.OwnerReadGrants = []production.Grant{grant}
			statements, err := scoped.GrantSQL()
			if err != nil {
				return err
			}
			for _, statement := range statements {
				if _, err := tx.Exec(ctx, statement); err != nil {
					return err
				}
			}
		}
	}
	return nil
}
