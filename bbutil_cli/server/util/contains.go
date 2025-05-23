package util

func Contains(elements []string, v string) bool {
	for _, s := range elements {
		if v == s {
			return true
		}
	}
	return false
}
