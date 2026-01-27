package util

import "strconv"

func StringToInt(v string) int {
	s, err := strconv.Atoi(v)
	Check(err)
	return s
}

func IntToString(v int) string {
	return strconv.Itoa(v)
}
