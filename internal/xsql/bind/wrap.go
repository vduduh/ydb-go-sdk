package bind

import (
	_ "embed"
	"regexp"
	"strings"
	"text/template"

	"github.com/ydb-platform/ydb-go-sdk/v3/internal/allocator"
	"github.com/ydb-platform/ydb-go-sdk/v3/internal/meta"
	"github.com/ydb-platform/ydb-go-sdk/v3/internal/xerrors"
	"github.com/ydb-platform/ydb-go-sdk/v3/table"
)

var (
	//go:embed wrap.yql
	wrapQuery    string
	wrapTemplate = template.Must(template.New("").Funcs(template.FuncMap{
		"Lines": func(s string) (ss []string) {
			if s == "" {
				return nil
			}
			return strings.Split(s, "\n")
		},
	}).Parse(wrapQuery))
)

type (
	wrapTemplateParams struct {
		Version     string
		SourceQuery string
		Pragmas     []string
		Declares    []Declare
		FinalQuery  string
	}
)

func (b Bindings) wrap(query string, re *regexp.Regexp, replace func(string) string, params *table.QueryParameters) (
	_ string, _ *table.QueryParameters, err error,
) {
	if !b.AllowBindParams && b.TablePathPrefix == "" {
		return query, params, nil
	}

	return wrap(query, func() string {
		if b.AllowBindParams && re != nil && replace != nil {
			return re.ReplaceAllStringFunc(query, replace)
		}
		return query
	}(), params, b.pragmas())
}

func wrap(sourceQuery, finalQuery string, params *table.QueryParameters, pragmas []string) (
	_ string, _ *table.QueryParameters, err error,
) {
	var declares = declares(params)

	if sourceQuery == finalQuery && len(pragmas) == 0 && len(declares) == 0 {
		return finalQuery, params, nil
	}

	p := wrapTemplateParams{
		Version:     "v" + meta.Version,
		SourceQuery: sourceQuery,
		Pragmas:     pragmas,
		Declares:    declares,
		FinalQuery:  finalQuery,
	}

	buffer := allocator.Buffers.Get()
	defer allocator.Buffers.Put(buffer)

	if err = wrapTemplate.Execute(buffer, p); err != nil {
		return "", nil, xerrors.WithStackTrace(err)
	}

	return buffer.String(), params, nil
}
