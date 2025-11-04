package flatfs

// keyIsValid returns true if the key is valid for flatfs.
// Allows keys that match [0-9a-z+-_=].
func keyIsValid(key string) bool {
	if len(key) < 1 {
		return false
	}
	for _, b := range key {
		if '0' <= b && b <= '9' {
			continue
		}
		if 'a' <= b && b <= 'z' {
			continue
		}
		switch b {
		case '+', '-', '_', '=':
			continue
		}
		return false
	}
	return true
}
