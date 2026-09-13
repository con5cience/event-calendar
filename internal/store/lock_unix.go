//go:build linux || darwin

package store

import (
	"fmt"
	"os"
	"syscall"
)

// Lock the existing directory inode, not a removable lock file. Closing the
// descriptor (including process death) releases the advisory lock.
func lockWriter(root *os.Root) (*os.File, error) {
	f, err := root.Open(".")
	if err != nil {
		return nil, err
	}
	if err = syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		f.Close()
		return nil, fmt.Errorf("cannot acquire writer lock: %w", err)
	}
	return f, nil
}
