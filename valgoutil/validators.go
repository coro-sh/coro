package valgoutil

import (
	"net"
	"net/url"

	"github.com/cohesivestack/valgo"
)

func HostPortValidator(hostPort string, nameAndTitle ...string) valgo.Validator {
	return valgo.String(hostPort, nameAndTitle...).Passing(func(hp string) bool {
		return isValidHostPort(hp)
	}, "must be a network address of the form 'host:port'")
}

func URLValidator(rawURL string, nameAndTitle ...string) valgo.Validator {
	return valgo.String(rawURL, nameAndTitle...).Passing(func(rawURL string) bool {
		return isValidURL(rawURL)
	}, "must be a valid URL")
}

func CORSValidator(origin string, nameAndTitle ...string) valgo.Validator {
	return valgo.String(origin, nameAndTitle...).Passing(func(origin string) bool {
		return isValidHostPort(origin) || isValidURL(origin)
	}, "must be a valid URL")
}

func NonEmptySliceValidator[T any](items []T, nameAndTitle ...string) valgo.Validator {
	return valgo.Any(items, nameAndTitle...).Passing(func(v any) bool {
		return len(v.([]T)) > 0
	}, "{{title}} must not be empty")
}

func isValidHostPort(hostPort string) bool {
	_, _, err := net.SplitHostPort(hostPort)
	return err == nil
}

func isValidURL(rawURL string) bool {
	parsedURL, err := url.ParseRequestURI(rawURL)
	if err != nil {
		return false
	}

	switch parsedURL.Scheme {
	case "http", "https", "ws", "wss", "nats":
	default:
		return false
	}

	return parsedURL.Host != ""
}
