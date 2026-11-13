package binary

import (
	"errors"
	"strconv"
	"strings"
)

// Stable auto-updates use three numeric components. Prerelease channel
// selection is checked separately; an automatic downgrade is never implicit.
func rejectOlderRelease(current, candidate string) error {
	parse := func(s string) ([3]uint64, error) {
		var result [3]uint64
		s = strings.TrimPrefix(s, "v")
		s, _, _ = strings.Cut(s, "-")
		parts := strings.Split(s, ".")
		if len(parts) != 3 {
			return result, errors.New("release version cannot be compared safely")
		}
		for i, p := range parts {
			if len(p) == 0 || (len(p) > 1 && p[0] == '0') {
				return result, errors.New("invalid release version")
			}
			n, err := strconv.ParseUint(p, 10, 64)
			if err != nil {
				return result, errors.New("invalid release version")
			}
			result[i] = n
		}
		return result, nil
	}
	a, err := parse(current)
	if err != nil {
		return err
	}
	b, err := parse(candidate)
	if err != nil {
		return err
	}
	for i := range a {
		if b[i] > a[i] {
			return nil
		}
		if b[i] < a[i] {
			return errors.New("automatic downgrade refused; use a reviewed recovery package")
		}
	}
	return nil
}
