package valgoutil

import (
	"fmt"
	"strings"

	"github.com/cohesivestack/valgo"
)

func GetDetails(err *valgo.Error) []string {
	if err == nil || err.Errors() == nil {
		return []string{}
	}

	var details []string
	for _, v := range err.Errors() {
		detail := fmt.Sprintf("%s: %s", v.Name(), fmt.Sprintf("[%s]", strings.Join(v.Messages(), ", ")))
		details = append(details, detail)
	}

	return details
}
