// Code generated by easyjson for marshaling/unmarshaling. DO NOT EDIT.

package garcon

import (
	json "encoding/json"
	easyjson "github.com/mailru/easyjson"
	jlexer "github.com/mailru/easyjson/jlexer"
	jwriter "github.com/mailru/easyjson/jwriter"
)

// suppress unused package warning
var (
	_ *json.RawMessage
	_ *jlexer.Lexer
	_ *jwriter.Writer
	_ easyjson.Marshaler
)

func easyjson8e52a332DecodeGithubComTealFinanceGarcon(in *jlexer.Lexer, out *versionInfo) {
	isTopLevel := in.IsStart()
	if in.IsNull() {
		if isTopLevel {
			in.Consumed()
		}
		in.Skip()
		return
	}
	in.Delim('{')
	for !in.IsDelim('}') {
		key := in.UnsafeFieldName(false)
		in.WantColon()
		if in.IsNull() {
			in.Skip()
			in.WantComma()
			continue
		}
		switch key {
		case "version":
			out.Version = string(in.String())
		case "short":
			out.Short = string(in.String())
		case "last_commit":
			out.LastCommit = string(in.String())
		case "ago":
			out.Ago = string(in.String())
		default:
			in.SkipRecursive()
		}
		in.WantComma()
	}
	in.Delim('}')
	if isTopLevel {
		in.Consumed()
	}
}
func easyjson8e52a332EncodeGithubComTealFinanceGarcon(out *jwriter.Writer, in versionInfo) {
	out.RawByte('{')
	first := true
	_ = first
	if in.Version != "" {
		const prefix string = ",\"version\":"
		first = false
		out.RawString(prefix[1:])
		out.String(string(in.Version))
	}
	if in.Short != "" {
		const prefix string = ",\"short\":"
		if first {
			first = false
			out.RawString(prefix[1:])
		} else {
			out.RawString(prefix)
		}
		out.String(string(in.Short))
	}
	if in.LastCommit != "" {
		const prefix string = ",\"last_commit\":"
		if first {
			first = false
			out.RawString(prefix[1:])
		} else {
			out.RawString(prefix)
		}
		out.String(string(in.LastCommit))
	}
	if in.Ago != "" {
		const prefix string = ",\"ago\":"
		if first {
			first = false
			out.RawString(prefix[1:])
		} else {
			out.RawString(prefix)
		}
		out.String(string(in.Ago))
	}
	out.RawByte('}')
}

// MarshalJSON supports json.Marshaler interface
func (v versionInfo) MarshalJSON() ([]byte, error) {
	w := jwriter.Writer{}
	easyjson8e52a332EncodeGithubComTealFinanceGarcon(&w, v)
	return w.Buffer.BuildBytes(), w.Error
}

// MarshalEasyJSON supports easyjson.Marshaler interface
func (v versionInfo) MarshalEasyJSON(w *jwriter.Writer) {
	easyjson8e52a332EncodeGithubComTealFinanceGarcon(w, v)
}

// UnmarshalJSON supports json.Unmarshaler interface
func (v *versionInfo) UnmarshalJSON(data []byte) error {
	r := jlexer.Lexer{Data: data}
	easyjson8e52a332DecodeGithubComTealFinanceGarcon(&r, v)
	return r.Error()
}

// UnmarshalEasyJSON supports easyjson.Unmarshaler interface
func (v *versionInfo) UnmarshalEasyJSON(l *jlexer.Lexer) {
	easyjson8e52a332DecodeGithubComTealFinanceGarcon(l, v)
}
