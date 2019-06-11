package binding

import (
	"net/http"
	"net/url"
	"reflect"
	"strconv"

	"github.com/bytedance/go-tagexpr"
	"github.com/henrylee2cn/goutil"
	"github.com/tidwall/gjson"
)

type paramInfo struct {
	fieldSelector  string
	structField    reflect.StructField
	tagInfos       []*tagInfo
	bindErrFactory func(failField, msg string) error
}

func (p *paramInfo) name(paramIn in) string {
	var name string
	for _, info := range p.tagInfos {
		if info.paramIn == json {
			name = info.paramName
			break
		}
	}
	if name == "" {
		return p.structField.Name
	}
	return name
}

func (p *paramInfo) getField(expr *tagexpr.TagExpr, initZero bool) (reflect.Value, error) {
	fh, found := expr.Field(p.fieldSelector)
	if found {
		v := fh.Value(initZero)
		if v.IsValid() {
			return v, nil
		}
	}
	return reflect.Value{}, nil
}

func (p *paramInfo) bindRawBody(info *tagInfo, expr *tagexpr.TagExpr, bodyBytes []byte) error {
	if len(bodyBytes) == 0 {
		if info.required {
			return info.requiredError
		}
		return nil
	}
	v, err := p.getField(expr, true)
	if err != nil || !v.IsValid() {
		return err
	}
	v = goutil.DereferenceValue(v)
	switch v.Kind() {
	case reflect.Slice:
		if v.Type().Elem().Kind() != reflect.Uint8 {
			return info.typeError
		}
		v.Set(reflect.ValueOf(bodyBytes))
		return nil
	case reflect.String:
		v.Set(reflect.ValueOf(goutil.BytesToString(bodyBytes)))
		return nil
	default:
		return info.typeError
	}
}

func (p *paramInfo) bindPath(info *tagInfo, expr *tagexpr.TagExpr, pathParams PathParams) (bool, error) {
	r, found := pathParams.Get(info.paramName)
	if !found {
		if info.required {
			return false, info.requiredError
		}
		return false, nil
	}
	return true, p.bindStringSlice(info, expr, []string{r})
}

func (p *paramInfo) bindQuery(info *tagInfo, expr *tagexpr.TagExpr, queryValues url.Values) (bool, error) {
	return p.bindMapStrings(info, expr, queryValues)
}

func (p *paramInfo) bindHeader(info *tagInfo, expr *tagexpr.TagExpr, header http.Header) (bool, error) {
	return p.bindMapStrings(info, expr, header)
}

func (p *paramInfo) bindCookie(info *tagInfo, expr *tagexpr.TagExpr, cookies []*http.Cookie) error {
	var r []string
	for _, c := range cookies {
		if c.Name == info.paramName {
			r = append(r, c.Value)
		}
	}
	if len(r) == 0 {
		if info.required {
			return info.requiredError
		}
		return nil
	}
	return p.bindStringSlice(info, expr, r)
}

func (p *paramInfo) bindOrRequireBody(info *tagInfo, expr *tagexpr.TagExpr, bodyCodec codec, bodyString string, postForm map[string][]string, checkOpt bool) (bool, error) {
	switch bodyCodec {
	case bodyForm:
		return p.bindMapStrings(info, expr, postForm)
	case bodyJSON:
		err := p.checkRequireJSON(info, expr, bodyString, checkOpt)
		return err == nil, err
	case bodyProtobuf:
		err := p.checkRequireProtobuf(info, expr, checkOpt)
		return err == nil, err
	default:
		return false, info.contentTypeError
	}
}

func (p *paramInfo) checkRequireProtobuf(info *tagInfo, expr *tagexpr.TagExpr, checkOpt bool) error {
	if checkOpt && !info.required {
		v, err := p.getField(expr, false)
		if err != nil || !v.IsValid() {
			return info.requiredError
		}
	}
	return nil
}

func (p *paramInfo) checkRequireJSON(info *tagInfo, expr *tagexpr.TagExpr, bodyString string, checkOpt bool) error {
	if checkOpt || info.required {
		r := gjson.Get(bodyString, info.namePath)
		if !r.Exists() {
			return info.requiredError
		}
		v, err := p.getField(expr, false)
		if err != nil || !v.IsValid() {
			return info.requiredError
		}
	}
	return nil
}

func (p *paramInfo) bindMapStrings(info *tagInfo, expr *tagexpr.TagExpr, values map[string][]string) (bool, error) {
	r, ok := values[info.paramName]
	if !ok || len(r) == 0 {
		if info.required {
			return false, info.requiredError
		}
		return false, nil
	}
	return true, p.bindStringSlice(info, expr, r)
}

func (p *paramInfo) bindStringSlice(info *tagInfo, expr *tagexpr.TagExpr, a []string) error {
	v, err := p.getField(expr, true)
	if err != nil || !v.IsValid() {
		return err
	}
	return setStringSlice(info, v, a)
}

func setStringSlice(info *tagInfo, v reflect.Value, a []string) error {
	v = goutil.DereferenceValue(v)
	switch v.Kind() {
	case reflect.String:
		v.Set(reflect.ValueOf(a[0]))
		return nil

	case reflect.Bool:
		bol, err := strconv.ParseBool(a[0])
		if err == nil {
			v.SetBool(bol)
			return nil
		}
		return nil

	case reflect.Float32:
		f, err := strconv.ParseFloat(a[0], 32)
		if err == nil {
			v.SetFloat(f)
			return nil
		}
		return nil
	case reflect.Float64:
		f, err := strconv.ParseFloat(a[0], 64)
		if err == nil {
			v.SetFloat(f)
			return nil
		}
		return nil

	case reflect.Int64, reflect.Int:
		i, err := strconv.ParseInt(a[0], 10, 64)
		if err == nil {
			v.SetInt(i)
			return nil
		}
	case reflect.Int32:
		i, err := strconv.ParseInt(a[0], 10, 32)
		if err == nil {
			v.SetInt(i)
			return nil
		}
	case reflect.Int16:
		i, err := strconv.ParseInt(a[0], 10, 16)
		if err == nil {
			v.SetInt(i)
			return nil
		}
	case reflect.Int8:
		i, err := strconv.ParseInt(a[0], 10, 8)
		if err == nil {
			v.SetInt(i)
			return nil
		}

	case reflect.Uint64, reflect.Uint:
		i, err := strconv.ParseUint(a[0], 10, 64)
		if err == nil {
			v.SetUint(i)
			return nil
		}
	case reflect.Uint32:
		i, err := strconv.ParseUint(a[0], 10, 32)
		if err == nil {
			v.SetUint(i)
			return nil
		}
	case reflect.Uint16:
		i, err := strconv.ParseUint(a[0], 10, 16)
		if err == nil {
			v.SetUint(i)
			return nil
		}
	case reflect.Uint8:
		i, err := strconv.ParseUint(a[0], 10, 8)
		if err == nil {
			v.SetUint(i)
			return nil
		}

	case reflect.Slice:
		tt := v.Type().Elem()
		vv, err := stringsToValue(tt.Kind(), a)
		if err == nil {
			v.Set(vv)
			return nil
		}
	}

	return info.typeError
}
