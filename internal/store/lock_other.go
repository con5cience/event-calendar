//go:build !linux && !darwin

package store

import (
	"fmt"
	"os"
)

func lockWriter(root *os.Root) (*os.File, error) {
	return nil, fmt.Errorf("local publication requires Linux or macOS filesystem locking")
}
