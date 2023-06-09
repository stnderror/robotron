package internal

import (
	"fmt"
	"os"
)

func MustGetEnv(key string) string {
	value := os.Getenv(key)
	if value == "" {
		panic(fmt.Errorf("%s not found", key))
	}
	return value
}
