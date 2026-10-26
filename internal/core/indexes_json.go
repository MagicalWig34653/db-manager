// SPDX-License-Identifier: AGPL-3.0-only
package core

import (
	"bytes"
	"encoding/json"
	"errors"
)

func (r *IndexesRequest) UnmarshalJSON(data []byte) error {
	type plain IndexesRequest
	value := plain(*r)
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&value); err != nil {
		return err
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return err
	}
	required := []string{"connection", "table", "action"}
	optional := []string{"timeout_ms", "confirm", "expected_plan_sha256"}
	switch value.Action {
	case "index_create", "primary_set":
		required = append(required, "index")
	case "index_alter":
		required = append(required, "name", "index")
	case "index_rename":
		required = append(required, "name", "new_name")
	case "index_drop", "foreign_key_drop":
		required = append(required, "name")
	case "foreign_key_create":
		required = append(required, "foreign_key")
	case "foreign_key_alter":
		required = append(required, "name", "foreign_key")
	case "primary_drop":
	default:
		return errors.New("core: unknown index action")
	}
	allowed := map[string]bool{}
	for _, key := range required {
		allowed[key] = true
		if _, ok := fields[key]; !ok {
			return errors.New("core: missing index field")
		}
	}
	for _, key := range optional {
		allowed[key] = true
	}
	for key, raw := range fields {
		if !allowed[key] || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
			return errors.New("core: irrelevant or null index field")
		}
	}
	if _, ok := fields["expected_plan_sha256"]; ok && value.ExpectedPlanSHA256 == "" {
		return errors.New("core: empty plan digest")
	}
	*r = IndexesRequest(value)
	return nil
}
