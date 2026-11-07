package bringup

import (
	"bufio"
	"errors"
	"io"
	"strings"
)

type ServiceIdentity struct{ Name, Target, Custody, CN, EKU, SANs string }

func ReadServiceRoster(r io.Reader) ([]ServiceIdentity, error) {
	var rows []ServiceIdentity
	seen := map[string]bool{}
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) != 6 || seen[fields[0]] {
			return nil, errors.New("invalid identity roster")
		}
		seen[fields[0]] = true
		retired := false
		for _, name := range []string{"receipt-authority", "guarantor", "relay-archive", "layerxd"} {
			if fields[0] == name || strings.HasPrefix(fields[0], name+"-") {
				retired = true
			}
		}
		if retired || fields[1] == "railway:router" || fields[1] == "railway:redis-router" {
			return nil, errors.New("retired identity owner")
		}
		rows = append(rows, ServiceIdentity{fields[0], fields[1], fields[2], fields[3], fields[4], fields[5]})
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, errors.New("empty identity roster")
	}
	return rows, nil
}
