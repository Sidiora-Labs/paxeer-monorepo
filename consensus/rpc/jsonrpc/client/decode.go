package client

import (
	"encoding/json"
	"errors"
	"fmt"

	tmjson "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/libs/json"
	rpctypes "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/jsonrpc/types"
)

func unmarshalResponseBytes(responseBytes []byte, expectedID string, result interface{}) error {
	// Read response.  If rpc/core/types is imported, the result will unmarshal
	// into the correct type.
	var response rpctypes.RPCResponse
	if err := tmjson.Unmarshal(responseBytes, &response); err != nil {
		return fmt.Errorf("unmarshaling response test: %w, bytes: %d", err, len(responseBytes))
	}

	if response.Error != nil {
		return response.Error
	}

	if got := response.ID(); got != expectedID {
		return fmt.Errorf("got response ID %q, wanted %q", got, expectedID)
	}

	// Unmarshal the RawMessage into the result.
	if err := tmjson.Unmarshal(response.Result, result); err != nil {
		return fmt.Errorf("error unmarshaling result: %w", err)
	}
	return nil
}

func unmarshalResponseBytesArray(responseBytes []byte, expectedIDs []string, results []interface{}) error {
	if len(expectedIDs) != len(results) {
		return fmt.Errorf("got %d result destinations, wanted %d", len(results), len(expectedIDs))
	}
	var responses []rpctypes.RPCResponse
	if err := json.Unmarshal(responseBytes, &responses); err != nil {
		return fmt.Errorf("unmarshaling responses: %w", err)
	} else if len(responses) != len(results) {
		return fmt.Errorf("got %d results, wanted %d", len(responses), len(results))
	}

	// Validate the entire ID set before modifying any result destinations.
	ids := make([]string, len(responses))
	byID := make(map[string]rpctypes.RPCResponse, len(responses))
	for i, resp := range responses {
		ids[i] = resp.ID()
		byID[resp.ID()] = resp
	}
	if err := validateResponseIDs(ids, expectedIDs); err != nil {
		return fmt.Errorf("wrong IDs: %w", err)
	}

	var errs []error
	for i, id := range expectedIDs {
		resp := byID[id]
		if resp.Error != nil {
			errs = append(errs, fmt.Errorf("response ID %q: %w", id, resp.Error))
			continue
		}
		if err := json.Unmarshal(resp.Result, results[i]); err != nil {
			errs = append(errs, fmt.Errorf("unmarshaling result %d (ID %q): %w", i, id, err))
		}
	}
	return errors.Join(errs...)
}

func validateResponseIDs(ids, expectedIDs []string) error {
	if len(ids) != len(expectedIDs) {
		return fmt.Errorf("got %d response IDs, wanted %d", len(ids), len(expectedIDs))
	}
	m := make(map[string]bool, len(expectedIDs))
	for _, id := range expectedIDs {
		if _, ok := m[id]; ok {
			return fmt.Errorf("duplicate expected ID: %q", id)
		}
		m[id] = false
	}

	for i, id := range ids {
		if seen, ok := m[id]; !ok {
			return fmt.Errorf("unexpected response ID %d: %q", i, id)
		} else if seen {
			return fmt.Errorf("duplicate response ID %d: %q", i, id)
		}
		m[id] = true
	}
	return nil
}
