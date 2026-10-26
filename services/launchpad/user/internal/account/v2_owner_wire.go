package account

import (
	"encoding/json"
	"fmt"

	v2 "github.com/Sidiora-Technologies/KindleLaunch/shared/interfaces/v2"
)

func AdaptV2Preferences(raw []byte, update bool) ([]byte, error) {
	envelope, data, err := v2OwnerEnvelope(raw)
	if err != nil {
		return nil, err
	}
	preferences, err := v2OwnerObject(data["preferences"])
	if err != nil {
		return nil, fmt.Errorf("preferences: %w", err)
	}
	categories, err := v2OwnerObject(preferences["notificationCategories"])
	if err != nil {
		return nil, fmt.Errorf("notificationCategories: %w", err)
	}
	for _, name := range []string{"trade", "priceAlert", "mention", "reward", "moderation", "community"} {
		if _, present := categories[name]; !present {
			categories[name] = json.RawMessage("false")
		}
	}
	categoryJSON, err := json.Marshal(categories)
	if err != nil {
		return nil, err
	}
	preferences["notificationCategories"] = categoryJSON
	projected := make(map[string]json.RawMessage)
	for _, name := range []string{"revision", "locale", "timezone", "notificationCategories", "channels", "quietHours", "analyticsConsent"} {
		if value, present := preferences[name]; present {
			projected[name] = value
		}
	}
	schema := "PreferencesReadResponse"
	if update {
		schema = "PreferencesUpdateResponse"
	}
	return v2OwnerResponse(schema, envelope, map[string]any{"preferences": projected})
}

func AdaptV2DeviceRegister(raw []byte) ([]byte, error) {
	envelope, data, err := v2OwnerEnvelope(raw)
	if err != nil {
		return nil, err
	}
	projected := make(map[string]json.RawMessage)
	for _, name := range []string{"deviceId", "state", "revision"} {
		if value, present := data[name]; present {
			projected[name] = value
		}
	}
	return v2OwnerResponse("DeviceRegisterResponse", envelope, projected)
}

func AdaptV2AccountJob(raw []byte, kind string, status bool) ([]byte, error) {
	var schema string
	switch kind {
	case "export":
		schema = "AccountExportResponse"
		if status {
			schema = "AccountExportStatusResponse"
		}
	case "deletion":
		schema = "AccountDeleteResponse"
		if status {
			schema = "AccountDeleteStatusResponse"
		}
	default:
		return nil, fmt.Errorf("unsupported account job kind %q", kind)
	}
	envelope, data, err := v2OwnerEnvelope(raw)
	if err != nil {
		return nil, err
	}
	job, err := v2OwnerObject(data["job"])
	if err != nil {
		return nil, fmt.Errorf("job: %w", err)
	}
	var jobKind string
	if err := json.Unmarshal(job["kind"], &jobKind); err != nil || jobKind != kind {
		return nil, fmt.Errorf("account job kind does not match %q", kind)
	}
	projected := map[string]json.RawMessage{"job": data["job"]}
	if kind == "export" && status {
		if value, present := data["downloadUrl"]; present {
			projected["downloadUrl"] = value
		}
	}
	if kind == "deletion" && !status {
		if value, present := data["retentionSummary"]; present {
			projected["retentionSummary"] = value
		}
	}
	return v2OwnerResponse(schema, envelope, projected)
}

func v2OwnerObject(raw []byte) (map[string]json.RawMessage, error) {
	var object map[string]json.RawMessage
	if err := json.Unmarshal(raw, &object); err != nil {
		return nil, err
	}
	if object == nil {
		return nil, fmt.Errorf("expected an object")
	}
	return object, nil
}

func v2OwnerEnvelope(raw []byte) (map[string]json.RawMessage, map[string]json.RawMessage, error) {
	envelope, err := v2OwnerObject(raw)
	if err != nil {
		return nil, nil, err
	}
	data, err := v2OwnerObject(envelope["data"])
	if err != nil {
		return nil, nil, fmt.Errorf("data: %w", err)
	}
	return envelope, data, nil
}

func v2OwnerResponse(schema string, envelope map[string]json.RawMessage, data any) ([]byte, error) {
	encoded, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}
	envelope["data"] = encoded
	raw, err := json.Marshal(envelope)
	if err != nil {
		return nil, err
	}
	if err := v2.ValidateJSON(schema, raw); err != nil {
		return nil, fmt.Errorf("%s: %w", schema, err)
	}
	return raw, nil
}
