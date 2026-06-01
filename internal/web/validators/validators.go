package validators

import (
	"mime"
	"net/textproto"
	"regexp"
	"strings"
)

var emailRe = regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)

func ValidateUserCreationInput(name, email string) bool {
	return name != "" && emailRe.MatchString(email)
}

func ValidatePDFContentType(contentType string) bool {
	if contentType == "" {
		return false
	}
	mediatype, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		mediatype = strings.ToLower(strings.TrimSpace(strings.Split(contentType, ";")[0]))
	}
	return strings.EqualFold(mediatype, "application/pdf")
}

func NormalizeHeaderKey(key string) string {
	return textproto.CanonicalMIMEHeaderKey(key)
}
