package brokerprovision

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/delivery/natsdriver"
	"github.com/nats-io/jwt/v2"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/nats-io/nkeys"
)

func repositoryRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs("../../../..")
	if err != nil {
		t.Fatal(err)
	}
	return root
}
func example(t *testing.T) Document {
	t.Helper()
	f, err := os.Open(filepath.Join(repositoryRoot(t), "deploy/launchpad/documents/values/broker.example.json"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	d, err := Read(f)
	if err != nil {
		t.Fatal(err)
	}
	return d
}
func encoded(t *testing.T, value any) []byte {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}
func smallDocument(t *testing.T) Document {
	t.Helper()
	d := example(t)
	for _, s := range d.Streams {
		if s.Name == "KINDLE_INDEXER_V1" {
			d.Streams = []Stream{s}
			break
		}
	}
	d.Durables = []natsdriver.ConsumerConfig{{Stream: d.Streams[0].Name, Durable: "bounded", Subject: "accounting.market.v1", Batch: 16, MaxAckPending: 64, MaxWaiting: 4, MaxDeliver: 10, FetchWait: 250 * time.Millisecond, AckWait: 30 * time.Second}}
	return d
}

func TestDocumentRefusesConsumerBoundsAndReferences(t *testing.T) {
	cases := []struct {
		name   string
		change func(*Document)
	}{
		{"stream_name", func(d *Document) { d.Durables[0].Stream = "bad.name" }},
		{"durable_name", func(d *Document) { d.Durables[0].Durable = "bad.name" }},
		{"empty_subject", func(d *Document) { d.Durables[0].Subject = "" }},
		{"wildcard_subject", func(d *Document) { d.Durables[0].Subject = "accounting.*" }},
		{"long_subject", func(d *Document) { d.Durables[0].Subject = strings.Repeat("a", 257) }},
		{"batch_low", func(d *Document) { d.Durables[0].Batch = 0 }},
		{"batch_high", func(d *Document) { d.Durables[0].Batch = 65 }},
		{"ack_pending_low", func(d *Document) { d.Durables[0].MaxAckPending = 15 }},
		{"ack_pending_high", func(d *Document) { d.Durables[0].MaxAckPending = 10001 }},
		{"waiting_low", func(d *Document) { d.Durables[0].MaxWaiting = 0 }},
		{"waiting_high", func(d *Document) { d.Durables[0].MaxWaiting = 33 }},
		{"deliver_low", func(d *Document) { d.Durables[0].MaxDeliver = 0 }},
		{"deliver_high", func(d *Document) { d.Durables[0].MaxDeliver = 101 }},
		{"fetch_low", func(d *Document) { d.Durables[0].FetchWait = 10*time.Millisecond - 1 }},
		{"fetch_high", func(d *Document) { d.Durables[0].FetchWait = time.Second + 1 }},
		{"ack_low", func(d *Document) { d.Durables[0].AckWait = time.Second - 1 }},
		{"ack_high", func(d *Document) { d.Durables[0].AckWait = time.Hour + 1 }},
		{"unlisted_stream", func(d *Document) { d.Durables[0].Stream = "UNLISTED" }},
		{"uncovered_subject", func(d *Document) { d.Durables[0].Subject = "other.event" }},
		{"duplicate_durable", func(d *Document) { d.Durables = append(d.Durables, d.Durables[0]) }},
		{"duplicate_stream", func(d *Document) { d.Streams = append(d.Streams, d.Streams[0]) }},
		{"no_streams", func(d *Document) { d.Streams = nil }},
		{"missing_durables", func(d *Document) { d.Durables = nil }},
		{"message_limit", func(d *Document) { d.Streams[0].Limits.MaxMessages = 0 }},
		{"byte_limit", func(d *Document) { d.Streams[0].Limits.MaxBytes = 0 }},
		{"age_limit", func(d *Document) { d.Streams[0].Limits.MaxAge = 0 }},
		{"size_limit", func(d *Document) { d.Streams[0].Limits.MaxMessageBytes = 0 }},
		{"consumer_limit", func(d *Document) { d.Streams[0].Limits.MaxConsumers = 0 }},
		{"consumer_limit_high", func(d *Document) { d.Streams[0].Limits.MaxConsumers = 10001 }},
		{"replicas_low", func(d *Document) { d.Streams[0].Limits.Replicas = 0 }},
		{"replicas_high", func(d *Document) { d.Streams[0].Limits.Replicas = 6 }},
		{"duplicates_low", func(d *Document) { d.Streams[0].Limits.DuplicateWindow = 0 }},
		{"duplicates_high", func(d *Document) { d.Streams[0].Limits.DuplicateWindow = d.Streams[0].Limits.MaxAge + 1 }},
		{"discard", func(d *Document) { d.Streams[0].Limits.Discard = "unknown" }},
		{"stream_subject", func(d *Document) { d.Streams[0].Subjects = []string{"accounting.>.v1"} }},
		{"stream_duplicate_subject", func(d *Document) { d.Streams[0].Subjects = append(d.Streams[0].Subjects, d.Streams[0].Subjects[0]) }},
		{"too_many_durables", func(d *Document) {
			d.Streams[0].Limits.MaxConsumers = 1
			c := d.Durables[0]
			c.Durable = "another"
			d.Durables = append(d.Durables, c)
		}},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			d := smallDocument(t)
			test.change(&d)
			if _, err := Read(bytes.NewReader(encoded(t, d))); err == nil {
				t.Fatal("invalid document admitted")
			}
		})
	}
}

func TestDocumentStrictJSONAndSubjectCoverage(t *testing.T) {
	raw := string(encoded(t, smallDocument(t)))
	for _, bad := range []string{
		raw + " {}", "null", "[]", strings.Replace(raw, `"streams":`, `"unknown":1,"streams":`, 1),
		strings.Replace(raw, `"streams":`, `"streams":[],"streams":`, 1),
		strings.Replace(raw, `"Batch":16`, `"Batch":16,"batch":16`, 1),
		strings.Replace(raw, `"Batch":16`, `"Batch":null`, 1),
		strings.Replace(raw, `"Batch":16`, `"Batch":1.5`, 1),
		strings.Replace(raw, `"Batch":16`, `"Batch":9223372036854775808`, 1),
		strings.Repeat(" ", MaxDocumentBytes+1),
	} {
		if _, err := Read(strings.NewReader(bad)); err == nil {
			t.Fatal("malformed document admitted")
		}
	}
	for _, test := range []struct {
		filter, subject string
		want            bool
	}{
		{"accounting.*.v1", "accounting.market.v1", true}, {"accounting.>", "accounting.market.v1", true},
		{"accounting.>", "accounting", false}, {"accounting.*", "accounting.market.v1", false},
		{"accounting.market.v1", "accounting.market.v1", true}, {">", "a", true}, {"a.>", "a.b", true},
		{"a..b", "a.b", false}, {"a.>.b", "a.x.b", false}, {"a.*", "a.*", false}, {"a.*", "a", false},
	} {
		if got := Covers(test.filter, test.subject); got != test.want {
			t.Fatalf("Covers(%q,%q)=%v", test.filter, test.subject, got)
		}
	}
	d := smallDocument(t)
	d.Streams[0].Subjects = []string{"accounting.*.v1"}
	if _, err := Read(bytes.NewReader(encoded(t, d))); err != nil {
		t.Fatal(err)
	}
	d.Durables = []natsdriver.ConsumerConfig{}
	if _, err := Read(bytes.NewReader(encoded(t, d))); err != nil {
		t.Fatal(err)
	}
	for _, upper := range []bool{false, true} {
		d = smallDocument(t)
		c := &d.Durables[0]
		if upper {
			c.Batch = 64
			c.MaxAckPending = 10000
			c.MaxWaiting = 32
			c.MaxDeliver = 100
			c.FetchWait = time.Second
			c.AckWait = time.Hour
		} else {
			c.Batch = 1
			c.MaxAckPending = 1
			c.MaxWaiting = 1
			c.MaxDeliver = 1
			c.FetchWait = 10 * time.Millisecond
			c.AckWait = time.Second
		}
		if _, err := Read(bytes.NewReader(encoded(t, d))); err != nil {
			t.Fatal(err)
		}
	}
}

func TestComposerExamplesMatchBrokerDocument(t *testing.T) {
	root := repositoryRoot(t)
	command := exec.Command("python3", filepath.Join("testdata", "composer_inventory.py"), root)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("composer inventory: %v\n%s", err, output)
	}
	var inventory struct {
		Streams  map[string][]string         `json:"streams"`
		Durables []natsdriver.ConsumerConfig `json:"durables"`
		Access   map[string]struct {
			Consumers       []natsdriver.ConsumerConfig `json:"consumers"`
			PublishSubjects []string                    `json:"publishSubjects"`
		} `json:"identities"`
	}
	if err = json.Unmarshal(output, &inventory); err != nil {
		t.Fatal(err)
	}
	searchIdentity, ok := inventory.Access["stats-search"]
	if !ok || len(searchIdentity.Consumers) != 3 || !reflect.DeepEqual(searchIdentity.PublishSubjects, []string{"search.invalidation.v2"}) {
		t.Fatal("stats search requires its three consumers and invalidation publication only")
	}
	policyConsumer := false
	for _, consumer := range searchIdentity.Consumers {
		if consumer.Subject == "social.policy" {
			policyConsumer = consumer.Stream == "SOCIAL_EVENTS" && consumer.Durable == "stats_search_social_policy"
		}
	}
	if !policyConsumer {
		t.Fatal("stats search identity is missing its social policy consumer")
	}
	d := example(t)
	if len(inventory.Streams) != len(d.Streams) {
		t.Fatalf("stream count: composer=%d broker=%d", len(inventory.Streams), len(d.Streams))
	}
	for _, s := range d.Streams {
		if !reflect.DeepEqual(s.Subjects, inventory.Streams[s.Name]) {
			t.Fatalf("subjects differ for %s: %v versus %v", s.Name, s.Subjects, inventory.Streams[s.Name])
		}
	}
	actual := map[string]natsdriver.ConsumerConfig{}
	for _, c := range d.Durables {
		actual[c.Stream+"/"+c.Durable] = c
	}
	expected := map[string]natsdriver.ConsumerConfig{}
	for _, c := range inventory.Durables {
		key := c.Stream + "/" + c.Durable
		if previous, ok := expected[key]; ok && previous != c {
			t.Fatalf("composer durable bounds disagree: %s", key)
		}
		expected[key] = c
	}
	if !reflect.DeepEqual(actual, expected) {
		t.Fatalf("composer and broker durable configurations differ: actual=%+v expected=%+v", actual, expected)
	}
	// The social event relay publishes policy and erasure through EventStream.
	socialRaw, err := os.ReadFile(filepath.Join(root, "deploy/launchpad/documents/values/social.example.json"))
	if err != nil {
		t.Fatal(err)
	}
	var social struct{ EventStream, ErasureStream, PolicyStream string }
	if err = json.Unmarshal(socialRaw, &social); err != nil {
		t.Fatal(err)
	}
	if social.EventStream != social.ErasureStream || social.EventStream != social.PolicyStream {
		t.Fatal("social event consumers must use the event publisher's stream")
	}
}

func TestCredentialBundleRequiresMatchingUserIdentity(t *testing.T) {
	account, err := nkeys.CreateAccount()
	if err != nil {
		t.Fatal(err)
	}
	defer account.Wipe()
	user, err := nkeys.CreateUser()
	if err != nil {
		t.Fatal(err)
	}
	defer user.Wipe()
	public, err := user.PublicKey()
	if err != nil {
		t.Fatal(err)
	}
	claims := jwt.NewUserClaims(public)
	token, err := claims.Encode(account)
	if err != nil {
		t.Fatal(err)
	}
	seed, err := user.Seed()
	if err != nil {
		t.Fatal(err)
	}
	raw, err := jwt.FormatUserConfig(token, seed)
	if err != nil {
		t.Fatal(err)
	}
	if err = validateCredentials(raw); err != nil {
		t.Fatal("valid signed user bundle refused")
	}
	other, err := nkeys.CreateUser()
	if err != nil {
		t.Fatal(err)
	}
	defer other.Wipe()
	otherSeed, err := other.Seed()
	if err != nil {
		t.Fatal(err)
	}
	mismatched, err := jwt.FormatUserConfig(token, otherSeed)
	if err != nil {
		t.Fatal(err)
	}
	if validateCredentials(mismatched) == nil {
		t.Fatal("mismatched signing seed admitted")
	}
	for _, bad := range [][]byte{nil, []byte("invalid"), []byte(token)} {
		if validateCredentials(bad) == nil {
			t.Fatal("invalid credentials admitted")
		}
	}
}

func TestConfigurationComparisonRetainsExactLimitsAndConsumerOptions(t *testing.T) {
	d := smallDocument(t)
	stream := d.Streams[0].BrokerConfig()
	stream.MaxMsgs = 1 << 53
	different := stream
	different.MaxMsgs++
	if equalStream(different, stream) {
		t.Fatal("distinct 64-bit stream limits compare equal")
	}
	reordered := stream
	reordered.Subjects = append([]string(nil), stream.Subjects...)
	for i, j := 0, len(reordered.Subjects)-1; i < j; i, j = i+1, j-1 {
		reordered.Subjects[i], reordered.Subjects[j] = reordered.Subjects[j], reordered.Subjects[i]
	}
	if !equalStream(reordered, stream) {
		t.Fatal("subject order changed stream equality")
	}
	consumer, err := d.Durables[0].BrokerConsumerConfig()
	if err != nil {
		t.Fatal(err)
	}
	for _, change := range []func(*jetstream.ConsumerConfig){
		func(c *jetstream.ConsumerConfig) { c.HeadersOnly = true },
		func(c *jetstream.ConsumerConfig) { c.MemoryStorage = true },
		func(c *jetstream.ConsumerConfig) { c.MaxRequestMaxBytes = 1024 },
		func(c *jetstream.ConsumerConfig) { c.DeliverSubject = "push.delivery" },
		func(c *jetstream.ConsumerConfig) { c.FilterSubjects = []string{c.FilterSubject} },
		func(c *jetstream.ConsumerConfig) { c.InactiveThreshold = time.Minute },
		func(c *jetstream.ConsumerConfig) { c.BackOff = []time.Duration{time.Second} },
	} {
		actual := consumer
		change(&actual)
		if equalConsumer(actual, consumer, 1) {
			t.Fatal("additional consumer option admitted")
		}
	}
}

func TestStatsSearchSocialPolicyDurable(t *testing.T) {
	root := repositoryRoot(t)
	read := func(name string, value any) {
		t.Helper()
		raw, err := os.ReadFile(filepath.Join(root, "deploy/launchpad/documents/values", name+".example.json"))
		if err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(raw, value); err != nil {
			t.Fatal(err)
		}
	}
	var stats struct {
		Phase2 struct {
			SearchConsumers []struct {
				Kind      string                    `json:"kind"`
				EventType string                    `json:"eventType"`
				Consumer  natsdriver.ConsumerConfig `json:"consumer"`
			} `json:"searchConsumers"`
		} `json:"phase2"`
	}
	var social struct{ PolicySubject, PolicyStream, PolicyDurable string }
	read("stats", &stats)
	read("social", &social)
	var search natsdriver.ConsumerConfig
	for _, sub := range stats.Phase2.SearchConsumers {
		if sub.Kind == "social_policy" {
			if sub.EventType != "social.policy.changed.v2" || sub.Consumer.Subject != social.PolicySubject || sub.Consumer.Stream != social.PolicyStream {
				t.Fatal("stats search policy binding differs from the social publisher")
			}
			search = sub.Consumer
		}
	}
	if search.Durable == "" || search.Durable == social.PolicyDurable {
		t.Fatal("stats search policy consumer is missing")
	}
	found := false
	for _, c := range example(t).Durables {
		if c.Stream == search.Stream && c.Durable == search.Durable {
			if c != search {
				t.Fatal("stats search policy consumer bounds differ")
			}
			found = true
		}
	}
	if !found {
		t.Fatal("stats search policy durable is missing from broker provisioning")
	}
}
