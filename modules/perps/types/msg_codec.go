package types

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
)

type paramsJSON Params

func (p Params) MarshalJSON() ([]byte, error) { return json.Marshal(paramsJSON(p)) }

func (p *Params) UnmarshalJSON(raw []byte) error { return json.Unmarshal(raw, (*paramsJSON)(p)) }

func (p Params) Marshal() ([]byte, error) { return p.MarshalJSON() }

func (p *Params) MarshalTo(data []byte) (int, error) {
	raw, err := p.Marshal()
	if err != nil {
		return 0, err
	}
	if len(data) < len(raw) {
		return 0, io.ErrShortBuffer
	}
	return copy(data, raw), nil
}

func (p *Params) Size() int {
	raw, err := p.Marshal()
	if err != nil {
		return 0
	}
	return len(raw)
}

func (p *Params) Unmarshal(data []byte) error {
	var next paramsJSON
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&next); err != nil {
		return err
	}
	var trailing interface{}
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err != nil {
			return err
		}
		return errors.New("params: trailing JSON value")
	}
	*p = Params(next)
	return nil
}

type marketJSON Market

func (m Market) MarshalJSON() ([]byte, error) { return json.Marshal(marketJSON(m)) }

func (m *Market) UnmarshalJSON(raw []byte) error { return json.Unmarshal(raw, (*marketJSON)(m)) }

func (m Market) Marshal() ([]byte, error) { return m.MarshalJSON() }

func (m *Market) MarshalTo(data []byte) (int, error) {
	raw, err := m.Marshal()
	if err != nil {
		return 0, err
	}
	if len(data) < len(raw) {
		return 0, io.ErrShortBuffer
	}
	return copy(data, raw), nil
}

func (m *Market) Size() int {
	raw, err := m.Marshal()
	if err != nil {
		return 0
	}
	return len(raw)
}

func (m *Market) Unmarshal(data []byte) error {
	var next marketJSON
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&next); err != nil {
		return err
	}
	var trailing interface{}
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err != nil {
			return err
		}
		return errors.New("market: trailing JSON value")
	}
	*m = Market(next)
	return nil
}
