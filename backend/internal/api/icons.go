package api

import (
	"bytes"
	"context"
	"errors"

	"github.com/getkin/kin-openapi/openapi3filter"

	"github.com/guided-traffic/dash/backend/internal/api/apigen"
	"github.com/guided-traffic/dash/backend/internal/icons"
	"github.com/guided-traffic/dash/backend/internal/problem"
)

// IconSource serves the icons the dashboard names; icons.Resolver does.
type IconSource interface {
	// CatalogIcon is the SVG of a bundled catalogue's icon; icons.ErrNoIcon
	// when there is no such icon at that version.
	CatalogIcon(pack, file string) ([]byte, error)
	// Blob is an icon of a ConfigMap or a URL and its media type;
	// icons.ErrNoIcon when none is kept under that name.
	Blob(file string) ([]byte, string, error)
}

// The headers of every served icon (docs/adr/0008): an address names one
// content for good, and an SVG opened on its own runs nothing on the
// dashboard's origin.
const (
	iconCacheControl = "public, max-age=31536000, immutable"
	iconPolicy       = "default-src 'none'; style-src 'unsafe-inline'; sandbox"
	iconNoSniff      = "nosniff"
)

func init() {
	// The response validation of the tests reads an icon's body as a file.
	openapi3filter.RegisterBodyDecoder("image/svg+xml", openapi3filter.FileBodyDecoder)
	openapi3filter.RegisterBodyDecoder("image/png", openapi3filter.FileBodyDecoder)
}

// GetBlobIcon answers an icon of a ConfigMap or a URL.
func (s *server) GetBlobIcon(_ context.Context, req apigen.GetBlobIconRequestObject) (apigen.GetBlobIconResponseObject, error) {
	if s.h.opts.Icons == nil {
		return nil, problem.New(problem.NotFound, "no icons are served")
	}
	data, contentType, err := s.h.opts.Icons.Blob(req.File)
	if errors.Is(err, icons.ErrNoIcon) {
		return nil, problem.New(problem.NotFound, "no such icon")
	}
	if err != nil {
		return nil, err
	}
	cache, policy, nosniff := iconCacheControl, iconPolicy, iconNoSniff
	headers := apigen.GetBlobIcon200ResponseHeaders{CacheControl: &cache, ContentSecurityPolicy: &policy, XContentTypeOptions: &nosniff}
	if contentType == "image/svg+xml" {
		return apigen.GetBlobIcon200ImagesvgXmlResponse{Body: bytes.NewReader(data), ContentLength: int64(len(data)), Headers: headers}, nil
	}
	return apigen.GetBlobIcon200ImagepngResponse{Body: bytes.NewReader(data), ContentLength: int64(len(data)), Headers: headers}, nil
}

// GetCatalogIcon answers an icon of a bundled catalogue.
func (s *server) GetCatalogIcon(_ context.Context, req apigen.GetCatalogIconRequestObject) (apigen.GetCatalogIconResponseObject, error) {
	if s.h.opts.Icons == nil {
		return nil, problem.New(problem.NotFound, "no icons are served")
	}
	data, err := s.h.opts.Icons.CatalogIcon(req.Pack, req.File)
	if errors.Is(err, icons.ErrNoIcon) {
		return nil, problem.New(problem.NotFound, "no such icon")
	}
	if err != nil {
		return nil, err
	}
	cache, policy, nosniff := iconCacheControl, iconPolicy, iconNoSniff
	return apigen.GetCatalogIcon200ImagesvgXmlResponse{
		Body:          bytes.NewReader(data),
		ContentLength: int64(len(data)),
		Headers:       apigen.GetCatalogIcon200ResponseHeaders{CacheControl: &cache, ContentSecurityPolicy: &policy, XContentTypeOptions: &nosniff},
	}, nil
}

// toIcon is a resolved icon in the API's shape.
func toIcon(i icons.Icon) apigen.Icon {
	out := apigen.Icon{Src: i.Src, Tint: i.Tint}
	if i.SrcDark != "" {
		out.SrcDark = &i.SrcDark
	}
	return out
}
