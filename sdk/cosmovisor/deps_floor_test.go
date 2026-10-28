package cosmovisor

import (
	"bufio"
	"os"
	"strings"
	"testing"

	"github.com/hashicorp/go-version"
)

func TestIndirectDependencyFloors(t *testing.T) {
	floors := map[string]string{
		"golang.org/x/crypto":                                   "v0.52.0",
		"github.com/klauspost/compress":                         "v1.18.7",
		"github.com/aws/aws-sdk-go-v2/service/s3":               "v1.97.3",
		"github.com/aws/aws-sdk-go-v2/aws/protocol/eventstream": "v1.7.8",
		"go.opentelemetry.io/otel/sdk":                          "v1.43.0",
		"google.golang.org/grpc":                                "v1.80.0",
		"golang.org/x/net":                                      "v0.52.0",
	}

	mod, err := os.Open("./go.mod")
	if err != nil {
		t.Fatal(err)
	}
	defer mod.Close()

	required := make(map[string]string)
	inRequire := false
	scanner := bufio.NewScanner(mod)
	for scanner.Scan() {
		line, _, _ := strings.Cut(scanner.Text(), "//")
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		if fields[0] == "require" {
			fields = fields[1:]
			if len(fields) == 1 && fields[0] == "(" {
				inRequire = true
				continue
			}
		} else if !inRequire {
			continue
		}
		if len(fields) == 1 && fields[0] == ")" {
			inRequire = false
			continue
		}
		if len(fields) != 2 {
			t.Fatalf("malformed require directive: %q", line)
		}
		required[fields[0]] = fields[1]
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}

	for module, floor := range floors {
		t.Run(module, func(t *testing.T) {
			pin, ok := required[module]
			if !ok {
				t.Fatalf("go.mod must require %s >= %s", module, floor)
			}
			got, err := version.NewSemver(pin)
			if err != nil {
				t.Fatalf("invalid version %q: %v", pin, err)
			}
			minimum, err := version.NewSemver(floor)
			if err != nil {
				t.Fatalf("invalid floor %q: %v", floor, err)
			}
			if got.LessThan(minimum) {
				t.Errorf("go.mod requires %s %s, want >= %s", module, pin, floor)
			}
		})
	}
}
