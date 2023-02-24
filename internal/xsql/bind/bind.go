package bind

import (
	"database/sql/driver"
	"fmt"
	"github.com/ydb-platform/ydb-go-sdk/v3/internal/xerrors"
	"github.com/ydb-platform/ydb-go-sdk/v3/table"
	"strconv"
	"strings"
	"sync"
)

var (
	pool paramsPool
)

type paramsPool struct {
	sync.Pool
}

func (p paramsPool) Get() []table.ParameterOption {
	pp := p.Pool.Get()
	if pp == nil {
		return []table.ParameterOption{}
	}
	return pp.([]table.ParameterOption)
}

func (p paramsPool) Put(pp []table.ParameterOption) {
	p.Pool.Put(pp[:0])
}

func (b Bindings) Bind(q string, args ...driver.NamedValue) (
	query string, _ *table.QueryParameters, _ error,
) {
	if len(args) == 0 {
		return wrap(q, q, nil, b.pragmas())
	}
	return bind(q, b.pragmas(), args...)
}

func bind(q string, pragmas []string, args ...driver.NamedValue) (
	query string, _ *table.QueryParameters, _ error,
) {
	switch t := queryType(q); t {
	case numericArgs:
		return bindNumeric(q, pragmas, args...)
	case positionalArgs:
		return bindPositional(q, pragmas, args...)
	case ydbArgs:
		return bindYdb(q, pragmas, args...)
	default:
		return "", nil, xerrors.WithStackTrace(fmt.Errorf("%s (type = %03b): %w", q, t, errUnknownQueryType))
	}
}

func bindPositional(q string, pragmas []string, args ...driver.NamedValue) (
	query string, _ *table.QueryParameters, _ error,
) {
	params, err := queryParams(args...)
	if err != nil {
		return "", nil, xerrors.WithStackTrace(err)
	}

	var (
		lines = strings.Split(q, "\n")
		num   = 0
	)
	for i := range lines {
		if !strings.HasPrefix(strings.TrimSpace(lines[i]), "--") {
			lines[i] = positionalArgsRe.ReplaceAllStringFunc(lines[i], func(s string) string {
				defer func() {
					num++
				}()
				return s[:1] + "$p" + strconv.Itoa(num)
			})
		}
	}

	return wrap(q, strings.Join(lines, "\n"), params, pragmas)
}

func bindNumeric(q string, pragmas []string, args ...driver.NamedValue) (
	query string, _ *table.QueryParameters, _ error,
) {
	params, err := queryParams(args...)
	if err != nil {
		return "", nil, xerrors.WithStackTrace(err)
	}

	lines := strings.Split(q, "\n")
	for i := range lines {
		if !strings.HasPrefix(strings.TrimSpace(lines[i]), "--") {
			lines[i] = numericArgsRe.ReplaceAllStringFunc(lines[i], func(s string) string {
				n, _ := strconv.Atoi(s[1:])
				return "$p" + strconv.Itoa(n-1)
			})
		}
	}

	return wrap(q, strings.Join(lines, "\n"), params, pragmas)
}

func bindYdb(q string, pragmas []string, args ...driver.NamedValue) (
	query string, _ *table.QueryParameters, _ error,
) {
	params, err := queryParams(args...)
	if err != nil {
		return "", nil, xerrors.WithStackTrace(err)
	}

	return wrap(q, q, params, pragmas)
}

func queryParams(values ...driver.NamedValue) (*table.QueryParameters, error) {
	pp := pool.Get()
	defer pool.Put(pp)

	for i, v := range values {
		switch x := v.Value.(type) {
		case *table.QueryParameters:
			if len(values) > 0 {
				return nil, xerrors.WithStackTrace(fmt.Errorf("%v: %w", values, ErrMultipleQueryParameters))
			}
			return x, nil
		case table.ParameterOption:
			pp = append(pp, x)
		default:
			value, err := ToValue(x)
			if err != nil {
				return nil, xerrors.WithStackTrace(err)
			}
			if v.Name == "" {
				v.Name = "$p" + strconv.Itoa(i)
			}
			pp = append(pp, table.ValueParam(v.Name, value))
		}
	}

	return table.NewQueryParameters(pp...), nil
}
