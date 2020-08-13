package openapi

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"sort"
	"strings"
	"unicode"

	"github.com/emicklei/go-restful"
	"github.com/yubo/golib/status"
	"github.com/yubo/golib/util"
	"k8s.io/klog/v2"
)

func Req2curl(req *http.Request, body []byte, inputFile, outputFile *string) string {
	buf := bytes.Buffer{}
	buf.WriteString("curl -X " + escapeShell(req.Method))

	if inputFile != nil {
		buf.WriteString(" -T " + escapeShell(*inputFile))
	}

	if outputFile != nil {
		buf.WriteString(" -o " + escapeShell(*outputFile))
	}

	if len(body) > 0 {
		buf.WriteString(" -d " + escapeShell(util.SubStr3(string(body), 512, -512)))
	}

	var keys []string
	for k := range req.Header {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		buf.WriteString(" -H " + escapeShell(fmt.Sprintf("%s: %s", k, strings.Join(req.Header[k], " "))))
	}

	buf.WriteString(" " + escapeShell(req.URL.String()))

	return buf.String()
}

func HttpRespPrint(out io.Writer, resp *http.Response, body []byte) {
	if out == nil || resp == nil {
		return
	}

	fmt.Fprintf(out, "[resp]\ncode: %d\n", resp.StatusCode)
	fmt.Fprintf(out, "header:\n")

	for k, v := range resp.Header {
		for _, v1 := range v {
			fmt.Fprintf(out, "  %s: %s\n", k, v1)
		}
	}

	if len(body) > 0 {
		fmt.Fprintf(out, "body:\n%s\n", string(body))
	}
}

func escapeShell(in string) string {
	return `'` + strings.Replace(in, `'`, `'\''`, -1) + `'`
}

// TODO: remove
func IsEmptyValue(v reflect.Value) bool {
	switch v.Kind() {
	case reflect.Array, reflect.Map, reflect.Slice, reflect.String:
		return v.Len() == 0
	case reflect.Bool:
		return !v.Bool()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return v.Int() == 0
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return v.Uint() == 0
	case reflect.Float32, reflect.Float64:
		return v.Float() == 0
	case reflect.Interface, reflect.Ptr:
		return v.IsNil()
	}
	return false
}

func HttpRedirectErr(resp *restful.Response, url string, err error) {
	if err != nil {
		resp.WriteError(http.StatusInternalServerError, err)
		return
	}
	resp.ResponseWriter.Header().Add("location", url)
	resp.ResponseWriter.WriteHeader(http.StatusFound)
}

func HttpWriteEntity(resp *restful.Response, in interface{}, err error) {
	if err != nil {
		HttpWriteErr(resp, err)
		return
	}

	if s, ok := in.(string); ok {
		resp.Write([]byte(s))
		return
	}

	resp.WriteEntity(in)
}

func HttpWrite(resp *restful.Response, data []byte, err error) {
	if err != nil {
		HttpWriteErr(resp, err)
		return
	}

	resp.Write(data)
}

func HttpWriteErr(resp *restful.Response, err error) {
	if err == nil {
		return
	}

	s, ok := status.FromError(err)
	if !ok {
		resp.WriteError(http.StatusBadRequest, err)
		return
	}

	if code := status.HTTPStatusFromCode(s.Code()); code != 200 {
		resp.WriteError(code, err)
	}

}

type tagOpt struct {
	name   string
	typ    string
	format string
	skip   bool
	inline bool
	inbody bool // just use cur interface{} as whole body
}

// `param:"(path|header|param|data)?(,inline|inbody)?"`
// `name:"keyName"`
// `json:"keyName"`
// func getTags(ff reflect.StructField) (name, paramType, format string, skip, inline bool) {
func getTagOpt(ff reflect.StructField) (opt tagOpt) {
	tag, _ := ff.Tag.Lookup("param")
	if tag == "-" {
		opt.skip = true
		return
	}

	if strings.HasSuffix(tag, ",inline") {
		opt.inline = true
		return
	}

	if strings.HasSuffix(tag, ",inbody") {
		opt.inbody = true
		return
	}

	tags := strings.Split(tag, ",")
	if len(tags) > 1 {
		opt.format = tags[1]
	}

	if tags[0] != "" {
		opt.typ = tags[0]
	} else {
		opt.typ = QueryType
	}

	var ok bool
	if tag, ok = ff.Tag.Lookup("name"); ok {
		opt.name = tag
		return
	}

	if opt.typ == DataType {
		if tag, ok = ff.Tag.Lookup("json"); ok {
			if tags := strings.Split(tag, ","); tags[0] != "" {
				opt.name = tags[0]
				return
			}
		}
	}

	// default rules
	switch opt.typ {
	case PathType:
		opt.name = strings.ToLower(ff.Name)
	default:
		opt.name = util.LowerCamelCasedName(ff.Name)
	}

	return
}

func Metadata(scopes ...string) (string, interface{}) {
	if len(scopes) == 1 && scopes[0] == OauthScopeNil {
		scopes = []string{}
	}
	return SecurityDefinitionKey, OAISecurity{
		Name:   OauthSecurityName,
		Scopes: scopes,
	}
}

// isVowel returns true if the rune is a vowel (case insensitive).
func isVowel(c rune) bool {
	vowels := []rune{'a', 'e', 'i', 'o', 'u'}
	for _, value := range vowels {
		if value == unicode.ToLower(c) {
			return true
		}
	}
	return false
}

func HttpRedirect(w http.ResponseWriter, url string) {
	w.Header().Add("location", url)
	w.WriteHeader(http.StatusFound)
}

// reflect

func rvInfo(rv reflect.Value) {
	if klog.V(5).Enabled() {
		klog.InfoDepth(1, fmt.Sprintf("isValid %v", rv.IsValid()))
		klog.InfoDepth(1, fmt.Sprintf("rv string %s kind %s", rv.String(), rv.Kind()))
	}
}
