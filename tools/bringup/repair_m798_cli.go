package bringup

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

func runM798Inspector(ctx context.Context, args []string, input io.Reader, output io.Writer) (bool, error) {
	invalid := errors.New("invalid bringup upgrade/xweb arguments")
	client := &http.Client{Timeout: 10 * time.Second}
	switch args[0] {
	case "upgrade-observe":
		if len(args) != 7 {
			return true, invalid
		}
		height, e := strconv.ParseUint(args[6], 10, 64)
		if e != nil || height == 0 {
			return true, invalid
		}
		ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
		defer cancel()
		observation, e := ObserveUpgrade(ctx, client, args[1], args[2], args[3], args[4], args[5], height, input)
		if e != nil {
			return true, e
		}
		return true, json.NewEncoder(output).Encode(observation)
	case "upgrade-state":
		if len(args) < 3 {
			return true, invalid
		}
		s, e := OpenUpgradeState(args[1])
		if e != nil {
			return true, e
		}
		defer s.Close()
		switch args[2] {
		case "begin":
			if len(args) != 8 {
				return true, invalid
			}
			height, e := strconv.ParseUint(args[6], 10, 64)
			if e != nil {
				return true, invalid
			}
			seconds, e := strconv.ParseInt(args[7], 10, 64)
			if e != nil {
				return true, invalid
			}
			plan := UpgradePlan{Version: args[3], Name: args[4], SHA256: args[5], Height: height, RecoverySeconds: seconds}
			raw, e := io.ReadAll(io.LimitReader(input, 1<<20))
			if e != nil {
				return true, e
			}
			for _, line := range strings.Split(strings.TrimSpace(string(raw)), "\n") {
				fields := strings.Fields(line)
				if len(fields) != 6 {
					return true, invalid
				}
				plan.Units = append(plan.Units, UpgradeUnit{fields[0], fields[1], fields[2], fields[3], fields[4], fields[5]})
			}
			return true, s.Begin(plan)
		case "next":
			if len(args) != 4 {
				return true, invalid
			}
			raw, e := io.ReadAll(io.LimitReader(input, 1<<20))
			if e != nil {
				return true, e
			}
			var observation UpgradeObservation
			decoder := json.NewDecoder(bytes.NewReader(raw))
			decoder.DisallowUnknownFields()
			if uniqueJSON(raw) != nil || decoder.Decode(&observation) != nil {
				return true, invalid
			}
			action, e := s.Next(args[3], observation, time.Now())
			if e == nil {
				_, e = fmt.Fprintln(output, action)
			}
			return true, e
		case "applied":
			if len(args) != 5 {
				return true, invalid
			}
			return true, s.Applied(args[3], args[4])
		case "fail":
			if len(args) != 5 {
				return true, invalid
			}
			return true, s.Failure(args[3], args[4], time.Now())
		}
		return true, invalid
	case "xweb-config":
		if len(args) < 2 {
			return true, invalid
		}
		if args[1] == "register" {
			if len(args) != 2 {
				return true, invalid
			}
			return true, RegisterXweb(ctx, client, os.Getenv("XWEB_GATEWAY_ENDPOINT"), os.Getenv("XWEB_REGISTER_KEY_FILE"), output)
		}
		o, e := XwebEnvironment(os.Getenv)
		if e != nil {
			return true, e
		}
		if args[1] == "prices" && len(args) == 2 {
			prices, e := XwebPrices(ctx, client, o)
			if e != nil {
				return true, e
			}
			for _, sym := range xwebSymbols {
				a := prices[sym]
				if _, e = fmt.Fprintln(output, sym, a.ID, a.Price); e != nil {
					return true, e
				}
			}
			return true, nil
		}
		if len(args) >= 5 && args[1] == "render" {
			return true, RenderXweb(ctx, client, o, args[2], args[3], args[4:], output)
		}
		return true, invalid
	}
	return false, nil
}
