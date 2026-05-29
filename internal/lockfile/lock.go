package lockfile

import "os"

type Lock struct {
	file *os.File
}

func (l *Lock) Release() error {
	if l == nil || l.file == nil {
		return nil
	}
	return l.release()
}
