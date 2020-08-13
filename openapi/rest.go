package openapi

import (
	"fmt"
)

const (
	// Accept or Content-Type used in Consumes() and/or Produces()
	MIME_JSON        = "application/json"
	MIME_XML         = "application/xml"
	MIME_TXT         = "text/plain"
	MIME_URL_ENCODED = "application/x-www-form-urlencoded"
	MIME_OCTET       = "application/octet-stream" // If Content-Type is not present in request, use the default

	PathType   = "path"
	QueryType  = "query"
	HeaderType = "header"
	DataType   = "data"

	MaxFormSize = int64(1<<63 - 1)
)

type OAISecurity struct {
	Name   string   // SecurityDefinition name
	Scopes []string // Scopes for oauth2
}

func (s *OAISecurity) Valid() error {
	switch s.Name {
	case "oauth2":
		return nil
	case "openIdConnect":
		return nil
	default:
		if len(s.Scopes) > 0 {
			return fmt.Errorf("oai Security scopes for scheme '%s' should be empty", s.Name)
		}
	}

	return nil
}
