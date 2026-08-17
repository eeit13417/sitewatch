// Package shared holds code genuinely common to api and ingestion — env
// loading and DB connection setup — so it exists in exactly one place
// instead of being copy-pasted into two Go modules. Referenced via a
// `replace` directive in each module's go.mod (see shared/README.md).
package shared

import (
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"

	"github.com/joho/godotenv"
)

// LoadRootEnv loads the repo-root .env so local dev doesn't need every var
// exported manually. Resolved relative to this file's own location, which
// works the same whether the caller is api/ or ingestion/ — both sit one
// directory below the repo root, same as this package.
func LoadRootEnv() {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		return
	}
	path := filepath.Join(filepath.Dir(thisFile), "..", ".env")
	if err := godotenv.Load(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		slog.Warn("could not load .env", "path", path, "error", err)
	}
}
