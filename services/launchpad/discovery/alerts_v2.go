package discovery

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	v2 "github.com/Sidiora-Technologies/KindleLaunch/shared/interfaces/v2"
	"github.com/google/uuid"
)

func RuleFromV2AlertCreate(raw []byte, ownerAccountID string) (AlertRule, error) {
	if err := v2.ValidateJSON("AlertCreateBody", raw); err != nil {
		return AlertRule{}, fmt.Errorf("%w: alert create body: %v", ErrInvalid, err)
	}
	var body v2.AlertCreateBody
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&body); err != nil {
		return AlertRule{}, fmt.Errorf("%w: alert create body: %v", ErrInvalid, err)
	}
	rule := AlertRule{
		ID: uuid.NewString(), OwnerAccountID: ownerAccountID,
		PoolAddress: body.PoolAddress, Metric: "price", Direction: body.Direction,
		Threshold:  string(body.Threshold.Numerator) + "/" + string(body.Threshold.Denominator),
		Hysteresis: "0", Cooldown: time.Duration(body.CooldownSeconds) * time.Second,
		Channels: []string{"inapp"}, Revision: 1,
	}
	if err := rule.Validate(); err != nil {
		return AlertRule{}, err
	}
	return rule, nil
}

func AdaptV2AlertCreate(rule AlertRule, wallet string, threshold v2.InputPrice, requestID string, generatedAt time.Time) (v2.AlertCreateResponse, error) {
	if err := rule.Validate(); err != nil {
		return v2.AlertCreateResponse{}, err
	}
	if rule.Metric != "price" || rule.Cooldown%time.Second != 0 {
		return v2.AlertCreateResponse{}, ErrInvalid
	}
	raw, err := json.Marshal(threshold)
	if err != nil {
		return v2.AlertCreateResponse{}, fmt.Errorf("%w: threshold: %v", ErrInvalid, err)
	}
	if err := v2.ValidateJSON("InputPrice", raw); err != nil {
		return v2.AlertCreateResponse{}, fmt.Errorf("%w: threshold: %v", ErrInvalid, err)
	}
	value, err := exact(string(threshold.Numerator) + "/" + string(threshold.Denominator))
	if err != nil {
		return v2.AlertCreateResponse{}, err
	}
	ruleValue, err := exact(rule.Threshold)
	if err != nil || ruleValue.Cmp(value) != 0 {
		return v2.AlertCreateResponse{}, ErrInvalid
	}
	var price v2.Price
	if err := json.Unmarshal(raw, &price); err != nil {
		return v2.AlertCreateResponse{}, fmt.Errorf("%w: threshold: %v", ErrInvalid, err)
	}
	state := "enabled"
	if rule.Disabled {
		state = "disabled"
	}
	revision := v2.PositiveUInt256(strconv.FormatUint(rule.Revision, 10))
	response := v2.AlertCreateResponse{
		RequestId: requestID, GeneratedAt: generatedAt.UTC().Format("2006-01-02T15:04:05.000Z"),
		Quality: "complete", Revision: &revision,
		Data: v2.AlertCreateData{Alert: v2.Alert{
			AlertId: rule.ID, Wallet: wallet, PoolAddress: rule.PoolAddress,
			Direction: rule.Direction, Threshold: price,
			CooldownSeconds: v2.SafeInteger(rule.Cooldown / time.Second), State: state,
		}},
	}
	raw, err = json.Marshal(response)
	if err != nil {
		return v2.AlertCreateResponse{}, fmt.Errorf("%w: alert create response: %v", ErrInvalid, err)
	}
	if err := v2.ValidateJSON("AlertCreateResponse", raw); err != nil {
		return v2.AlertCreateResponse{}, fmt.Errorf("%w: alert create response: %v", ErrInvalid, err)
	}
	return response, nil
}
