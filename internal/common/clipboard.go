package common

import (
	"github.com/atotto/clipboard"
)

func SetClipboardValue(value string) error {
	return clipboard.WriteAll(value)
}

func GetClipboardValue() (string, error) {
	value, err := clipboard.ReadAll()

	if err != nil {
		//	LogInfo()
		return "", err
	}

	return value, nil
}
