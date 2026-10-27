package pipeline

import (
	"encoding/base64"
	"strings"
)

func mustB64(dataURL string) []byte {
	b, err := base64.StdEncoding.DecodeString(dataURL[strings.Index(dataURL, ",")+1:])
	if err != nil {
		panic(err)
	}
	return b
}
