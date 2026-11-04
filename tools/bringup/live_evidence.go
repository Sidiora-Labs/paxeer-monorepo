package bringup

import (
	"bufio"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"
)

type RegionalDeployment struct {
	ID        string `json:"id"`
	Status    string `json:"status"`
	Instances []struct {
		ID     string `json:"id"`
		Status string `json:"status"`
	} `json:"instances"`
	Meta struct {
		ServiceManifest struct {
			Deploy struct {
				MultiRegionConfig map[string]struct {
					NumReplicas int `json:"numReplicas"`
				} `json:"multiRegionConfig"`
			} `json:"deploy"`
		} `json:"serviceManifest"`
	} `json:"meta"`
}

func ReadRegionalDeployment(r io.Reader) (RegionalDeployment, error) {
	var doc struct {
		LatestDeployment RegionalDeployment `json:"latestDeployment"`
	}
	if json.NewDecoder(io.LimitReader(r, 1<<20)).Decode(&doc) != nil {
		return RegionalDeployment{}, errors.New("deployment unreadable")
	}
	d := doc.LatestDeployment
	if !safeIdentity(d.ID) || d.Status != "SUCCESS" || len(d.Meta.ServiceManifest.Deploy.MultiRegionConfig) < 2 {
		return d, errors.New("successful multi-region deployment required")
	}
	want := 0
	for region, config := range d.Meta.ServiceManifest.Deploy.MultiRegionConfig {
		if !safeIdentity(region) || config.NumReplicas < 1 {
			return d, errors.New("invalid region configuration")
		}
		want += config.NumReplicas
	}
	seen := map[string]bool{}
	for _, i := range d.Instances {
		if !safeIdentity(i.ID) || seen[i.ID] || (i.Status != "SUCCESS" && i.Status != "RUNNING") {
			return d, errors.New("running distinct deployment instances required")
		}
		seen[i.ID] = true
	}
	if len(d.Instances) != want {
		return d, errors.New("deployment replica count differs from running instances")
	}
	return d, nil
}

func safeIdentity(s string) bool {
	if s == "" || len(s) > 128 {
		return false
	}
	for _, c := range s {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '-' || c == '_') {
			return false
		}
	}
	return true
}

type RegionalResponse struct {
	Instance   string          `json:"instance"`
	Deployment string          `json:"deployment"`
	Region     string          `json:"region"`
	Body       json.RawMessage `json:"body"`
}

func ValidateRegionalResponses(d RegionalDeployment, r io.Reader, now time.Time, output io.Writer) error {
	var responses []RegionalResponse
	if json.NewDecoder(io.LimitReader(r, 4<<20)).Decode(&responses) != nil {
		return errors.New("regional responses unreadable")
	}
	instances := map[string]bool{}
	for _, i := range d.Instances {
		instances[i.ID] = false
	}
	regions := map[string]int{}
	// Buffer claims: no partial regional success before coverage is proven.
	var claims strings.Builder
	for _, response := range responses {
		used, exists := instances[response.Instance]
		if !exists || used || response.Deployment != d.ID {
			return errors.New("response belongs to an unselected or duplicate instance")
		}
		if _, ok := d.Meta.ServiceManifest.Deploy.MultiRegionConfig[response.Region]; !ok {
			return errors.New("response region is undeclared")
		}
		if err := ValidateGatewayReadiness(strings.NewReader(string(response.Body)), now); err != nil {
			return err
		}
		instances[response.Instance] = true
		regions[response.Region]++
		fmt.Fprintf(&claims, "pass readyz deployment=%s instance=%s region=%s service=layerx-gateway\n", d.ID, response.Instance, response.Region)
	}
	for _, used := range instances {
		if !used {
			return errors.New("selected deployment instance did not answer")
		}
	}
	for region, config := range d.Meta.ServiceManifest.Deploy.MultiRegionConfig {
		if regions[region] != config.NumReplicas {
			return errors.New("running regional replica count differs from declaration")
		}
	}
	_, err := io.WriteString(output, claims.String())
	return err
}

func ValidateHumanChild(r io.Reader, status int, output io.Writer) error {
	required := map[string]bool{"live": false, "preflight": false, "plan": false}
	summary, complete, failed := false, false, false
	scanner := bufio.NewScanner(io.LimitReader(r, 1<<20))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "pass ") || strings.HasPrefix(line, "fail ") {
			fmt.Fprintln(output, line)
			fields := strings.Fields(line)
			if fields[0] == "fail" {
				failed = true
			}
			if len(fields) >= 3 {
				if _, ok := required[fields[1]]; ok {
					if required[fields[1]] {
						failed = true
					}
					required[fields[1]] = fields[0] == "pass"
				}
			}
		}
		if line == "check-live: all checks passed" {
			summary = true
		}
		if line == "pass human-evidence checks=live,preflight,plan" {
			complete = true
		}
	}
	if scanner.Err() != nil {
		failed = true
	}
	for _, present := range required {
		if !present {
			failed = true
		}
	}
	if status != 0 || failed || !summary || !complete {
		fmt.Fprintf(output, "fail human-child exit=%d evidence=incomplete-or-failed\n", status)
		return errors.New("required Human child gate failed")
	}
	return nil
}

type IndexerState struct {
	Database      string  `json:"database"`
	Start         uint64  `json:"start"`
	RetainedStart uint64  `json:"retained_start"`
	Earliest      uint64  `json:"earliest"`
	Cutover       uint64  `json:"cutover"`
	Done          bool    `json:"done"`
	Backfill      *uint64 `json:"backfill"`
	Live          *uint64 `json:"live"`
	BackfillHash  string  `json:"backfill_hash"`
	LiveHash      string  `json:"live_hash"`
}

func ValidateIndexerState(r io.Reader, output io.Writer) error {
	var s IndexerState
	if json.NewDecoder(io.LimitReader(r, 1<<20)).Decode(&s) != nil || !s.Done || s.Start == 0 || s.Start != s.RetainedStart || s.Earliest != s.Start || s.Cutover < s.Start || s.Backfill == nil || s.Live == nil || *s.Backfill != s.Cutover || *s.Live < s.Cutover || s.BackfillHash == "" || s.LiveHash == "" || (*s.Live == *s.Backfill && s.LiveHash != s.BackfillHash) {
		return errors.New("indexer durable backfill/live evidence is incomplete or inconsistent")
	}
	data, err := base64.StdEncoding.DecodeString(s.Database)
	if err != nil {
		return errors.New("indexer database snapshot encoding invalid")
	}
	cursors, err := ReadSQLiteCursors(data)
	if err != nil {
		return err
	}
	if cursors["backfill_cursors"].Position != *s.Backfill || cursors["cursors"].Position != *s.Live || cursors["backfill_cursors"].Hash != s.BackfillHash || cursors["cursors"].Hash != s.LiveHash {
		return errors.New("indexer cursor report differs from real SQLite snapshot")
	}
	_, err = fmt.Fprintf(output, "pass backfill chain=paxeer start=%d cutover=%d backfill=%d live=%d database=consistent\n", s.Start, s.Cutover, *s.Backfill, *s.Live)
	return err
}

func ValidateFreshDelivery(events, deliveries io.Reader, endpoint, subject string, started uint64, now uint64, output io.Writer) error {
	var es []struct {
		ID       string `json:"id"`
		Subject  string `json:"subject"`
		Occurred uint64 `json:"occurred_at"`
	}
	var ds []struct {
		Delivery string `json:"delivery"`
		Endpoint string `json:"endpoint"`
		Event    string `json:"event"`
		Subject  string `json:"subject"`
		Created  uint64 `json:"created_at"`
		State    struct {
			State   string `json:"state"`
			At      uint64 `json:"at"`
			Status  uint16 `json:"status"`
			Attempt uint32 `json:"attempt"`
		} `json:"state"`
	}
	if !safeIdentity(endpoint) || subject == "" || started == 0 || now < started || json.NewDecoder(io.LimitReader(events, 4<<20)).Decode(&es) != nil || json.NewDecoder(io.LimitReader(deliveries, 4<<20)).Decode(&ds) != nil {
		return errors.New("fresh webhook evidence unreadable")
	}
	fresh := map[string]bool{}
	for _, e := range es {
		if e.Subject == subject && e.ID != "" && e.Occurred >= started && e.Occurred <= now {
			fresh[e.ID] = true
		}
	}
	sort.Slice(ds, func(i, j int) bool { return ds[i].Delivery < ds[j].Delivery })
	for _, d := range ds {
		if d.Endpoint == endpoint && d.Subject == subject && fresh[d.Event] && d.Delivery != "" && d.Created >= started && d.Created <= now && d.State.State == "delivered" && d.State.At >= d.Created && d.State.At <= now && d.State.Attempt > 0 && d.State.Status >= 200 && d.State.Status < 300 {
			_, err := fmt.Fprintf(output, "pass delivery endpoint=%s event=%s delivery=%s subject=%s state=delivered receiver-http=%d\n", endpoint, d.Event, d.Delivery, subject, d.State.Status)
			return err
		}
	}
	return errors.New("no fresh source event and matching receiver acknowledgment")
}
