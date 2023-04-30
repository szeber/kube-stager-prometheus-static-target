package helper

func StringInStringSlice(needle string, haystack []string) bool {
	for _, v := range haystack {
		if needle == v {
			return true
		}
	}

	return false
}
