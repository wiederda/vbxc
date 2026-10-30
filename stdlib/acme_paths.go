package stdlib

import (
	"fmt"
	"os"
	"path/filepath"
)

type Paths struct {
	AccountKey   string
	AccountState string
	CSR          string
	Output       string
}

func NewPaths(
	accountKey string,
	accountState string,
	csr string,
	output string,
) Paths {
	return Paths{
		AccountKey:   filepath.Clean(accountKey),
		AccountState: filepath.Clean(accountState),
		CSR:          filepath.Clean(csr),
		Output:       filepath.Clean(output),
	}
}

func EnsureParentDirectory(path string) error {
	dir := filepath.Dir(path)

	if dir == "." || dir == "" {
		return nil
	}

	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf(
			"Verzeichnis %q konnte nicht erstellt werden: %w",
			dir,
			err,
		)
	}

	return nil
}
