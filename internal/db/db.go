package db

import (
	"fmt"
	"strings"
	"time"
)

var DBConnTimeout = 1 * time.Minute

func MakeColumnString(names []string) string {
	return strings.Join(names, ",")
}

func MakeValuesString(names []string) string {
	result := make([]string, len(names))
	for i := range names {
		result[i] = fmt.Sprintf("$%d", i+1)
	}

	return strings.Join(result, ",")
}
