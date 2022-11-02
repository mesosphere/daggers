package utils

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
)

func SHA256SumFile(file string) (string, error) {
	f, err := os.Open(file)
	if err != nil {
		return "", fmt.Errorf("failed to open file %q: %w", file, err)
	}

	h := sha256.New()

	if _, err := io.Copy(h, f); err != nil {
		return "", fmt.Errorf("failed to create sha256 sum of file %q: %w", file, err)
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}
