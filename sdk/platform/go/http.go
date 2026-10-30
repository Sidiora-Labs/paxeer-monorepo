package layerx

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/binary"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"unicode/utf8"
)

const maximumHTTPResponseBytes = 8 * 1024 * 1024

type RequestAuthorizer func(*http.Request) error

type HumanHTTPTransport struct {
	baseURL    *url.URL
	client     *http.Client
	authorizer RequestAuthorizer
}

func NewHumanHTTPTransport(baseURL string, client *http.Client, authorizer RequestAuthorizer) (*HumanHTTPTransport, error) {
	parsed, err := url.Parse(baseURL)
	if err != nil || parsed.Host == "" || parsed.User != nil || (parsed.Scheme != "https" && parsed.Scheme != "http") {
		return nil, newSDKError(ErrorInvalidArgument, RetryNever)
	}
	if parsed.Scheme == "http" && !loopbackHost(parsed.Hostname()) {
		return nil, newSDKError(ErrorInvalidArgument, RetryNever)
	}
	if client == nil {
		client = &http.Client{}
	}
	boundedClient := *client
	boundedClient.CheckRedirect = func(_ *http.Request, _ []*http.Request) error {
		return http.ErrUseLastResponse
	}
	return &HumanHTTPTransport{baseURL: parsed, client: &boundedClient, authorizer: authorizer}, nil
}

func loopbackHost(host string) bool {
	if host == "localhost" {
		return true
	}
	address := net.ParseIP(host)
	return address != nil && address.IsLoopback()
}

func NewLayerXKeyAuthorizer(keyID string, secret string) (RequestAuthorizer, error) {
	if !validLayerXKeyID(keyID) || len(secret) != len("lxp_live_")+64 || !strings.HasPrefix(secret, "lxp_live_") || !canonicalLowerHex(secret[len("lxp_live_"):], 32) {
		return nil, newSDKError(ErrorInvalidArgument, RetryNever)
	}
	authorization := "LayerX-Key " + keyID + ":" + secret
	return func(request *http.Request) error {
		if request == nil {
			return newSDKError(ErrorInvalidArgument, RetryNever)
		}
		request.Header.Set("Authorization", authorization)
		return nil
	}, nil
}

func validLayerXKeyID(value string) bool {
	if value == "" || len(value) > 64 {
		return false
	}
	for index := range value {
		byteValue := value[index]
		if !(byteValue >= 'a' && byteValue <= 'z' || byteValue >= 'A' && byteValue <= 'Z' || byteValue >= '0' && byteValue <= '9' || byteValue == '-' || byteValue == '_') {
			return false
		}
	}
	return true
}

func validLayerXAuthorization(value string) bool {
	credential := strings.TrimPrefix(value, "LayerX-Key ")
	if credential == value {
		return false
	}
	keyID, secret, ok := strings.Cut(credential, ":")
	return ok && validLayerXKeyID(keyID) && len(secret) == len("lxp_live_")+64 && strings.HasPrefix(secret, "lxp_live_") && canonicalLowerHex(secret[len("lxp_live_"):], 32)
}

func (transport *HumanHTTPTransport) request(ctx context.Context, call TransportCall) (*http.Request, error) {
	if transport == nil {
		return nil, newSDKError(ErrorUnavailableCapability, RetryNever)
	}
	if call.Plane != PlaneHuman {
		return nil, newSDKError(ErrorUnavailableCapability, RetryNever)
	}
	metadata, ok := HumanOperation(call.Operation).Metadata()
	if !ok {
		return nil, newSDKError(ErrorInvalidArgument, RetryNever)
	}
	path := metadata.Path
	for name, value := range call.PathParameters {
		if name == "" || value == "" {
			return nil, newSDKError(ErrorInvalidArgument, RetryNever)
		}
		path = strings.ReplaceAll(path, "{"+name+"}", url.PathEscape(value))
	}
	if strings.ContainsAny(path, "{}") {
		return nil, newSDKError(ErrorInvalidArgument, RetryNever)
	}
	target := *transport.baseURL
	target.Path = strings.TrimRight(target.Path, "/") + path
	target.RawQuery = cloneValues(call.Query).Encode()

	var body io.Reader
	if metadata.Request != "Empty" {
		body = bytes.NewReader(call.Request)
	}
	request, err := http.NewRequestWithContext(ctx, metadata.Method, target.String(), body)
	if err != nil {
		return nil, newSDKError(ErrorInvalidArgument, RetryNever)
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("User-Agent", "layerx-go/0.1.0")
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	if call.IdempotencyKey.valid() {
		request.Header.Set("Idempotency-Key", call.IdempotencyKey.String())
	}
	if transport.authorizer != nil {
		if err := transport.authorizer(request); err != nil {
			return nil, transportError(ctx, err)
		}
	}
	return request, nil
}

func (transport *HumanHTTPTransport) Call(ctx context.Context, call TransportCall) (json.RawMessage, error) {
	request, err := transport.request(ctx, call)
	if err != nil {
		return nil, err
	}
	response, err := transport.client.Do(request)
	if err != nil {
		return nil, transportError(ctx, err)
	}
	defer response.Body.Close()
	limited := io.LimitReader(response.Body, maximumHTTPResponseBytes+1)
	encoded, err := io.ReadAll(limited)
	if err != nil {
		return nil, transportError(ctx, err)
	}
	if len(encoded) > maximumHTTPResponseBytes {
		return nil, newSDKError(ErrorDecodeFailure, RetryNever)
	}
	var envelope humanEnvelope
	if err := json.Unmarshal(encoded, &envelope); err != nil || envelope.Trace == "" {
		return nil, newSDKError(ErrorDecodeFailure, RetryNever)
	}
	if envelope.OK {
		if len(envelope.Result) == 0 || envelope.Error != nil || response.StatusCode < 200 || response.StatusCode >= 300 {
			return nil, newSDKError(ErrorDecodeFailure, RetryNever)
		}
		return append(json.RawMessage(nil), envelope.Result...), nil
	}
	if envelope.Error == nil || envelope.Error.Code == "" || response.StatusCode >= 200 && response.StatusCode < 300 {
		return nil, newSDKError(ErrorDecodeFailure, RetryNever)
	}
	return nil, envelope.Error.sdkError(envelope.Trace)
}

func decodeProgramAgentEnvelope(status int, encoded []byte, operation string) (json.RawMessage, *SDKError) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(encoded, &fields); err != nil {
		return nil, newSDKError(ErrorDecodeFailure, RetryNever)
	}
	if (programLifecycleOrdinals[operation] != 0 || operation == "program.receipt") && status >= 200 && status < 300 && exactFields(fields, "result") {
		return append(json.RawMessage(nil), fields["result"]...), nil
	}
	if programLifecycleOrdinals[operation] != 0 && status >= 400 && status < 500 && exactFields(fields, "error") {
		var refusal map[string]json.RawMessage
		var code, retry string
		if decodeStrict(fields["error"], &refusal) != nil || !(exactFields(refusal, "code", "retry") || exactFields(refusal, "code", "retry", "retry_after_seconds")) || json.Unmarshal(refusal["code"], &code) != nil || len(code) == 0 || len(code) > 256 || json.Unmarshal(refusal["retry"], &retry) != nil {
			return nil, newSDKError(ErrorDecodeFailure, RetryNever)
		}
		result := newSDKError(ErrorCoreRejection, RetryNever)
		result.ServiceCode = code
		if status == 401 || status == 403 {
			result.Code = ErrorCapabilityRefusal
		}
		if status == 409 {
			result.Code = ErrorIdempotencyConflict
		}
		if status == 429 {
			result.Code = ErrorRateLimit
		}
		if retry == "after" {
			var seconds uint64
			if json.Unmarshal(refusal["retry_after_seconds"], &seconds) != nil || seconds == 0 || seconds > ^uint64(0)/1000 {
				return nil, newSDKError(ErrorDecodeFailure, RetryNever)
			}
			milliseconds := seconds * 1000
			result.Retry = RetryAfter
			result.RetryAfterMillis = &milliseconds
		} else if retry != "never" || !exactFields(refusal, "code", "retry") {
			return nil, newSDKError(ErrorDecodeFailure, RetryNever)
		}
		return nil, result
	}
	if _, failed := fields["class"]; failed {
		if status >= 200 && status < 300 || len(fields) != 5 {
			return nil, newSDKError(ErrorDecodeFailure, RetryNever)
		}
		return nil, decodeProgramAgentError(fields)
	}
	if status < 200 || status >= 300 || len(fields) != 3 {
		return nil, newSDKError(ErrorDecodeFailure, RetryNever)
	}
	var requestID string
	if err := json.Unmarshal(fields["request_id"], &requestID); err != nil || requestID == "" || len(requestID) > 256 {
		return nil, newSDKError(ErrorDecodeFailure, RetryNever)
	}
	value := fields["value"]
	if len(value) == 0 || bytes.Equal(value, []byte("null")) {
		return nil, newSDKError(ErrorDecodeFailure, RetryNever)
	}
	if !acceptedProgramVerification(operation, value, fields["verification_status"]) {
		return nil, newSDKError(ErrorVerificationFailure, RetryNever)
	}
	return append(json.RawMessage(nil), value...), nil
}

func acceptedProgramVerification(operation string, value json.RawMessage, encoded json.RawMessage) bool {
	var verification map[string]json.RawMessage
	if decodeStrict(encoded, &verification) != nil || verification == nil {
		return false
	}
	if operation == "program.discover" || operation == "program.interface" {
		return exactProgramUnverified(verification, "server_side_receipt_verification_only")
	}
	var result map[string]json.RawMessage
	_ = decodeStrict(value, &result)
	var resultState string
	_ = json.Unmarshal(result["state"], &resultState)
	if (operation == "program.call" || operation == "program.receipt" || operation == "program.activity") && (resultState == "unknown" || resultState == "pending") {
		return exactProgramUnverified(verification, "receipt_pending")
	}
	if !exactFields(verification, "state", "level") {
		return false
	}
	var state string
	var level string
	return json.Unmarshal(verification["state"], &state) == nil && json.Unmarshal(verification["level"], &level) == nil && state == "Achieved" && level == "SequencerSigned"
}

func exactProgramUnverified(value map[string]json.RawMessage, reason string) bool {
	if !exactFields(value, "state", "requested", "achieved", "reason") {
		return false
	}
	var state string
	var requested string
	var achieved string
	var actualReason string
	return json.Unmarshal(value["state"], &state) == nil && json.Unmarshal(value["requested"], &requested) == nil && json.Unmarshal(value["achieved"], &achieved) == nil && json.Unmarshal(value["reason"], &actualReason) == nil && state == "Unverified" && requested == "SequencerSigned" && achieved == "Unverified" && actualReason == reason
}

func decodeProgramAgentError(fields map[string]json.RawMessage) *SDKError {
	var class AgentErrorClass
	var retriability AgentRetriability
	var requestID string
	var reason string
	if json.Unmarshal(fields["class"], &class) != nil || !class.Valid() || json.Unmarshal(fields["retriability"], &retriability) != nil || !retriability.Valid() || json.Unmarshal(fields["request_id"], &requestID) != nil || requestID == "" || len(requestID) > 256 || json.Unmarshal(fields["reason"], &reason) != nil || reason == "" || len(reason) > 256 {
		return newSDKError(ErrorDecodeFailure, RetryNever)
	}
	var resultCode *int32
	protocolResult := fields["protocol_result_code"]
	if len(protocolResult) == 0 {
		return newSDKError(ErrorDecodeFailure, RetryNever)
	}
	if !bytes.Equal(protocolResult, []byte("null")) {
		var value int32
		if json.Unmarshal(protocolResult, &value) != nil {
			return newSDKError(ErrorDecodeFailure, RetryNever)
		}
		resultCode = &value
	}
	code := ErrorInternalFault
	switch class {
	case AgentErrorTransportFailure:
		code = ErrorTransportFailure
	case AgentErrorDeadline:
		code = ErrorDeadline
	case AgentErrorProtocolIncompatibility:
		code = ErrorProtocolIncompatible
	case AgentErrorUnavailableCapability:
		code = ErrorUnavailableCapability
	case AgentErrorCoreRejection:
		code = ErrorCoreRejection
	case AgentErrorVerificationFailure:
		code = ErrorVerificationFailure
	case AgentErrorPolicyRefusal:
		code = ErrorPolicyRefusal
	case AgentErrorCapabilityRefusal:
		code = ErrorCapabilityRefusal
	case AgentErrorBudgetRefusal:
		code = ErrorBudgetRefusal
	case AgentErrorRateLimit:
		code = ErrorRateLimit
	case AgentErrorIdempotencyConflict:
		code = ErrorIdempotencyConflict
	}
	retry := RetryNever
	if retriability == AgentRetryRetriable {
		retry = RetrySafe
	}
	result := newSDKError(code, retry)
	result.ServiceCode = string(class)
	result.RequestID = requestID
	result.ProtocolResultCode = resultCode
	return result
}

func canonicalLowerHex(value string, bytes int) bool {
	if len(value) != bytes*2 {
		return false
	}
	for index := range value {
		if !(value[index] >= '0' && value[index] <= '9' || value[index] >= 'a' && value[index] <= 'f') {
			return false
		}
	}
	return true
}

type humanEnvelope struct {
	OK     bool            `json:"ok"`
	Result json.RawMessage `json:"result"`
	Error  *humanAPIError  `json:"error"`
	Trace  string          `json:"trace"`
}

type humanAPIError struct {
	Code             HumanErrorCode `json:"code"`
	Retry            string         `json:"retry"`
	RetryAfterMillis *uint64        `json:"retry_after_ms,omitempty"`
}

func (apiError *humanAPIError) sdkError(trace string) *SDKError {
	code := ErrorCoreRejection
	switch apiError.Code {
	case HumanErrorRateLimited:
		code = ErrorRateLimit
	case HumanErrorUnavailable, HumanErrorUpstreamDegraded:
		code = ErrorTransportFailure
	case HumanErrorRefusedByPolicy:
		code = ErrorPolicyRefusal
	case HumanErrorRefusedByBudget, HumanErrorRefusedByLimit:
		code = ErrorBudgetRefusal
	case HumanErrorRefusedByCapability, HumanErrorForbidden, HumanErrorUnauthenticated, HumanErrorSessionExpired, HumanErrorStepUpRequired:
		code = ErrorCapabilityRefusal
	case HumanErrorRefusedByProtocol:
		code = ErrorCoreRejection
	case HumanErrorConflict:
		code = ErrorIdempotencyConflict
	}
	retry := RetryNever
	switch apiError.Retry {
	case "retriable":
		retry = RetrySafe
	case "retriable-after":
		retry = RetryAfter
	case "structural", "final":
		retry = RetryNever
	default:
		return newSDKError(ErrorDecodeFailure, RetryNever)
	}
	result := newSDKError(code, retry)
	result.ServiceCode = string(apiError.Code)
	result.RequestID = trace
	result.RetryAfterMillis = apiError.RetryAfterMillis
	return result
}

const agentEnvelopePath = "/v1/agent/rpc"
const agentEnvelopeVersion = 1
const maximumAgentEnvelopeRequestBytes = 1048576

type version int

type AgentSessionCredential struct {
	tenant     string
	sessionID  string
	tokenID    *SecretBytes
	generation uint64
}

func NewAgentSessionCredential(tenant string, sessionID string, tokenID *SecretBytes, generation uint64) (*AgentSessionCredential, error) {
	if tenant == "" || len(tenant) > 255 || strings.IndexByte(tenant, 0) >= 0 || !utf8.ValidString(tenant) || !canonicalLowerHex(sessionID, 32) || tokenID == nil {
		return nil, newSDKError(ErrorInvalidArgument, RetryNever)
	}
	if err := tokenID.Expose(func(value []byte) error {
		if !canonicalLowerHex(string(value), 32) {
			return newSDKError(ErrorInvalidArgument, RetryNever)
		}
		return nil
	}); err != nil {
		return nil, newSDKError(ErrorInvalidArgument, RetryNever)
	}
	return &AgentSessionCredential{tenant: tenant, sessionID: sessionID, tokenID: tokenID, generation: generation}, nil
}

func agentEnvelopeBootstrap(operation AgentOperation) bool {
	return operation == AgentOperationAgentRegister || operation == AgentOperationSessionOpen
}

type AgentEnvelopeHTTPTransport struct {
	endpoint   *HumanHTTPTransport
	credential *AgentSessionCredential
	path       string
}

const agentDaemonEnvelopePath = "/rpc"

func NewAgentDaemonEnvelopeHTTPTransport(baseURL string, client *http.Client, credential *AgentSessionCredential) (*AgentEnvelopeHTTPTransport, error) {
	endpoint, err := NewHumanHTTPTransport(baseURL, client, nil)
	if err != nil {
		return nil, err
	}
	if endpoint.baseURL.Scheme != "https" {
		return nil, newSDKError(ErrorInvalidArgument, RetryNever)
	}
	return &AgentEnvelopeHTTPTransport{endpoint: endpoint, credential: credential, path: agentDaemonEnvelopePath}, nil
}

func NewAgentEnvelopeHTTPTransport(baseURL string, client *http.Client, gatewayAuthorizer RequestAuthorizer, credential *AgentSessionCredential) (*AgentEnvelopeHTTPTransport, error) {
	if gatewayAuthorizer == nil {
		return nil, newSDKError(ErrorInvalidArgument, RetryNever)
	}
	endpoint, err := NewHumanHTTPTransport(baseURL, client, gatewayAuthorizer)
	if err != nil {
		return nil, err
	}
	return &AgentEnvelopeHTTPTransport{endpoint: endpoint, credential: credential, path: agentEnvelopePath}, nil
}

type agentEnvelopeCredential struct {
	Tenant     string `json:"tenant"`
	SessionID  string `json:"session_id"`
	TokenID    string `json:"token_id"`
	Generation string `json:"generation"`
}

type agentEnvelope struct {
	Version        version                  `json:"version"`
	RequestID      string                   `json:"request_id"`
	Operation      string                   `json:"operation"`
	Request        json.RawMessage          `json:"request"`
	Credential     *agentEnvelopeCredential `json:"credential"`
	IdempotencyKey *string                  `json:"idempotency_key"`
}

func newAgentEnvelopeRequestID() (string, error) {
	var encoded [8]byte
	if _, err := rand.Read(encoded[:]); err != nil {
		return "", err
	}
	return strconv.FormatUint(binary.BigEndian.Uint64(encoded[:]), 10), nil
}

func (transport *AgentEnvelopeHTTPTransport) encode(call TransportCall, requestID string) ([]byte, error) {
	operation := AgentOperation(call.Operation)
	if call.Plane != PlaneAgent || !operation.Valid() || len(call.PathParameters) != 0 || len(call.Query) != 0 {
		return nil, newSDKError(ErrorInvalidArgument, RetryNever)
	}
	var fields map[string]json.RawMessage
	if len(call.Request) == 0 || !utf8.Valid(call.Request) || decodeStrict(call.Request, &fields) != nil || fields == nil {
		return nil, newSDKError(ErrorInvalidArgument, RetryNever)
	}
	envelope := agentEnvelope{Version: agentEnvelopeVersion, RequestID: requestID, Operation: call.Operation, Request: call.Request}
	if operation.RequiresIdempotency() {
		if !canonicalProgramKey(call.IdempotencyKey) {
			return nil, newSDKError(ErrorIdempotencyRequired, RetryNever)
		}
		key := call.IdempotencyKey.String()
		envelope.IdempotencyKey = &key
	} else if call.IdempotencyKey.valid() {
		return nil, newSDKError(ErrorInvalidArgument, RetryNever)
	}
	var encoded []byte
	encode := func() error {
		var err error
		encoded, err = json.Marshal(envelope)
		return err
	}
	if agentEnvelopeBootstrap(operation) {
		if err := encode(); err != nil {
			return nil, newSDKError(ErrorInvalidArgument, RetryNever)
		}
	} else {
		if transport.credential == nil {
			return nil, newSDKError(ErrorCapabilityRefusal, RetryNever)
		}
		credential := transport.credential
		if err := credential.tokenID.Expose(func(token []byte) error {
			envelope.Credential = &agentEnvelopeCredential{Tenant: credential.tenant, SessionID: credential.sessionID, TokenID: string(token), Generation: strconv.FormatUint(credential.generation, 10)}
			return encode()
		}); err != nil {
			return nil, newSDKError(ErrorInvalidArgument, RetryNever)
		}
	}
	if len(encoded) > maximumAgentEnvelopeRequestBytes {
		clear(encoded)
		return nil, newSDKError(ErrorInvalidArgument, RetryNever)
	}
	return encoded, nil
}

func (transport *AgentEnvelopeHTTPTransport) request(ctx context.Context, body []byte) (*http.Request, error) {
	target := *transport.endpoint.baseURL
	target.Path = strings.TrimRight(target.Path, "/") + transport.path
	target.RawQuery = ""
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, target.String(), bytes.NewReader(body))
	if err != nil {
		return nil, newSDKError(ErrorInvalidArgument, RetryNever)
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("User-Agent", "layerx-go/0.1.0")
	if transport.endpoint.authorizer == nil {
		return request, nil
	}
	if err := transport.endpoint.authorizer(request); err != nil {
		return nil, transportError(ctx, err)
	}
	if len(request.Header.Values("Authorization")) != 1 || !validLayerXAuthorization(request.Header.Get("Authorization")) {
		return nil, newSDKError(ErrorCapabilityRefusal, RetryNever)
	}
	for _, name := range []string{"LayerX-Tenant", "LayerX-Agent", "LayerX-Key"} {
		if len(request.Header.Values(name)) != 0 {
			return nil, newSDKError(ErrorCapabilityRefusal, RetryNever)
		}
	}
	return request, nil
}

func (transport *AgentEnvelopeHTTPTransport) Call(ctx context.Context, call TransportCall) (json.RawMessage, error) {
	if transport == nil || transport.endpoint == nil {
		return nil, newSDKError(ErrorUnavailableCapability, RetryNever)
	}
	requestID, err := newAgentEnvelopeRequestID()
	if err != nil {
		return nil, newSDKError(ErrorInternalFault, RetryNever)
	}
	body, err := transport.encode(call, requestID)
	if err != nil {
		return nil, err
	}
	defer clear(body)
	request, err := transport.request(ctx, body)
	if err != nil {
		return nil, err
	}
	mutation := AgentOperation(call.Operation).RequiresIdempotency()
	// Transport ambiguity on a mutation is Unknown: the outcome must be reconciled, never resent automatically.
	ambiguous := func(fallback *SDKError) *SDKError {
		if mutation {
			return newSDKError(ErrorUnknownOutcome, RetryUnknownOutcome)
		}
		return fallback
	}
	response, err := transport.endpoint.client.Do(request)
	if err != nil {
		return nil, ambiguous(transportError(ctx, err))
	}
	defer response.Body.Close()
	encoded, err := io.ReadAll(io.LimitReader(response.Body, maximumHTTPResponseBytes+1))
	if err != nil {
		return nil, ambiguous(transportError(ctx, err))
	}
	if len(encoded) > maximumHTTPResponseBytes {
		return nil, ambiguous(newSDKError(ErrorDecodeFailure, RetryNever))
	}
	value, decodeError := decodeAgentEnvelopeResponse(response.StatusCode, encoded, requestID)
	if decodeError != nil && (decodeError.Code == ErrorDecodeFailure || decodeError.Code == ErrorVerificationFailure || decodeError.Code == ErrorTransportFailure && decodeError.RequestID == "") {
		return nil, ambiguous(decodeError)
	}
	if decodeError != nil {
		return nil, decodeError
	}
	return value, nil
}

var agentEnvelopeRungs = map[string]int{
	"submitted": 1, "receipted": 2, "committed": 3,
}

func decodeAgentEnvelopeResponse(status int, encoded []byte, requestID string) (json.RawMessage, *SDKError) {
	// A non-JSON body or an edge 502/503 without a typed ApiError is transport ambiguity; well-formed JSON outside the schema is a decode failure.
	if !utf8.Valid(encoded) || !json.Valid(encoded) {
		return nil, newSDKError(ErrorTransportFailure, RetrySafe)
	}
	var fields map[string]json.RawMessage
	strictErr := decodeStrict(encoded, &fields)
	if _, typed := fields["class"]; (status == http.StatusBadGateway || status == http.StatusServiceUnavailable) && (strictErr != nil || !typed) {
		return nil, newSDKError(ErrorTransportFailure, RetrySafe)
	}
	if strictErr != nil || fields == nil {
		return nil, newSDKError(ErrorDecodeFailure, RetryNever)
	}
	var echoed string
	if json.Unmarshal(fields["request_id"], &echoed) != nil {
		return nil, newSDKError(ErrorDecodeFailure, RetryNever)
	}
	if _, failed := fields["class"]; failed {
		// A refusal issued before the envelope parsed carries request_id "0".
		if echoed != requestID && echoed != "0" || status >= 200 && status < 300 || !exactFields(fields, "class", "protocol_result_code", "retriability", "request_id", "reason") {
			return nil, newSDKError(ErrorDecodeFailure, RetryNever)
		}
		result := decodeProgramAgentError(fields)
		if result.Code == ErrorDecodeFailure {
			return nil, result
		}
		var reason string
		_ = json.Unmarshal(fields["reason"], &reason)
		result.ServiceCode = reason
		return nil, result
	}
	if echoed != requestID || status != http.StatusOK || !exactFields(fields, "request_id", "value", "verification_status") {
		return nil, newSDKError(ErrorDecodeFailure, RetryNever)
	}
	value := fields["value"]
	if !acceptedAgentEnvelopeVerification(fields["verification_status"]) {
		return nil, newSDKError(ErrorVerificationFailure, RetryNever)
	}
	return append(json.RawMessage(nil), value...), nil
}

func acceptedAgentEnvelopeVerification(encoded json.RawMessage) bool {
	var verification map[string]json.RawMessage
	if decodeStrict(encoded, &verification) != nil || verification == nil {
		return false
	}
	var state string
	if json.Unmarshal(verification["state"], &state) != nil {
		return false
	}
	switch state {
	case "achieved":
		var rung string
		return exactFields(verification, "state", "rung") && json.Unmarshal(verification["rung"], &rung) == nil && agentEnvelopeRungs[rung] != 0
	case "unverified":
		var requested, achieved, reason string
		return exactFields(verification, "state", "requested", "achieved", "reason") &&
			json.Unmarshal(verification["requested"], &requested) == nil && agentEnvelopeRungs[requested] != 0 &&
			json.Unmarshal(verification["achieved"], &achieved) == nil && agentEnvelopeRungs[achieved] != 0 &&
			agentEnvelopeRungs[achieved] < agentEnvelopeRungs[requested] &&
			json.Unmarshal(verification["reason"], &reason) == nil && reason != "" && len(reason) <= 256
	default:
		return false
	}
}
