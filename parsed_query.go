/*
Copyright 2017 Google Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package sqlparser

import (
	"bytes"
	"fmt"

	"github.com/clearblade/sqlparser/dependency/querypb"
	"github.com/clearblade/sqlparser/dependency/sqltypes"
)

// ParsedQuery represents a parsed query where
// bind locations are precompued for fast substitutions.
type ParsedQuery struct {
	Query         string
	bindLocations []bindLocation
}

type bindLocation struct {
	offset, length int
}

// NewParsedQuery returns a ParsedQuery of the ast.
func NewParsedQuery(node SQLNode) *ParsedQuery {
	buf := NewTrackedBuffer(nil)
	buf.Myprintf(nil, "%v", node)
	return buf.ParsedQuery()
}

// GenerateQuery generates a query by substituting the specified
// bindVariables. The extras parameter specifies special parameters
// that can perform custom encoding.
func (pq *ParsedQuery) GenerateQuery(bindVariables map[string]*querypb.BindVariable, extras map[string]Encodable) ([]byte, error) {
	if len(pq.bindLocations) == 0 {
		return []byte(pq.Query), nil
	}
	buf := bytes.NewBuffer(make([]byte, 0, len(pq.Query)))
	current := 0
	for _, loc := range pq.bindLocations {
		buf.WriteString(pq.Query[current:loc.offset])
		name := pq.Query[loc.offset : loc.offset+loc.length]
		if encodable, ok := extras[name]; ok {
			encodable.EncodeSQL(buf)
		} else {
			supplied, err := FetchBindVar(name, bindVariables)
			if err != nil {
				return nil, err
			}
			EncodeValue(buf, supplied)
		}
		current = loc.offset + loc.length
	}
	buf.WriteString(pq.Query[current:])
	return buf.Bytes(), nil
}

// EncodeValue encodes one bind variable value into the query.
func EncodeValue(buf *bytes.Buffer, value *querypb.BindVariable) {
	if value.Type != querypb.Type_TUPLE {
		// Since we already check for TUPLE, we don't expect an error.
		v, _ := sqltypes.BindVariableToValue(value)
		v.EncodeSQL(buf)
		return
	}

	// It's a TUPLE.
	buf.WriteByte('(')
	for i, bv := range value.Values {
		if i != 0 {
			buf.WriteString(", ")
		}
		sqltypes.ProtoToValue(bv).EncodeSQL(buf)
	}
	buf.WriteByte(')')
}

// FetchBindVar resolves the bind variable by fetching it from bindVariables.
func FetchBindVar(name string, bindVariables map[string]*querypb.BindVariable) (val *querypb.BindVariable, err error) {
	if name[0] == ':' {
		name = name[1:]
	}
	supplied, ok := bindVariables[name]
	if !ok {
		return nil, fmt.Errorf("missing bind var %s", name)
	}
	if supplied.Type == querypb.Type_TUPLE && len(supplied.Values) == 0 {
		return nil, fmt.Errorf("empty list supplied for %s", name)
	}
	return supplied, nil
}
