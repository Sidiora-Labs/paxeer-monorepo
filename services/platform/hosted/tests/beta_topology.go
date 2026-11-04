package hostedtests

import (
	"fmt"
	"net"
	"regexp"
	"strconv"
	"strings"
)

var serviceLabel = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?$`)

func ResolveServiceHost(host, callerNamespace string, externalHosts []string) (namespace, name, kind string, err error) {
	host = strings.TrimSuffix(strings.ToLower(host), ".")
	for _, external := range externalHosts {
		if host == strings.TrimSuffix(strings.ToLower(external), ".") {
			return "", "", "offcluster", nil
		}
	}
	if ip := net.ParseIP(host); ip != nil {
		if ip.IsLoopback() {
			return "", "", "loopback", nil
		}
		return "", "", "", fmt.Errorf("external IP %s must be explicitly declared", host)
	}
	parts := strings.Split(host, ".")
	for _, part := range parts {
		if !serviceLabel.MatchString(part) {
			return "", "", "", fmt.Errorf("invalid Service DNS name %s", host)
		}
	}
	switch {
	case len(parts) == 1 && serviceLabel.MatchString(callerNamespace):
		return callerNamespace, parts[0], "service", nil
	case len(parts) == 2:
		return parts[1], parts[0], "service", nil
	case len(parts) == 3 && parts[2] == "svc":
		return parts[1], parts[0], "service", nil
	case len(parts) == 5 && parts[2] == "svc" && parts[3] == "cluster" && parts[4] == "local":
		return parts[1], parts[0], "service", nil
	default:
		return "", "", "", fmt.Errorf("host %s is neither a Kubernetes Service nor an explicitly declared external host", host)
	}
}

type TopologyWorkload struct {
	Kind   string               `json:"kind"`
	Name   string               `json:"name"`
	NS     string               `json:"ns"`
	Labels map[string]string    `json:"labels"`
	Ports  map[string][2]string `json:"ports"`
}

type TopologyEdge struct {
	Caller TopologyWorkload `json:"caller"`
	Source string           `json:"source"`
	Host   string           `json:"host"`
	Port   string           `json:"port"`
	Listed bool             `json:"listed"`
}

type TopologyService struct {
	NS           string            `json:"ns"`
	Name         string            `json:"name"`
	Selector     map[string]string `json:"selector"`
	Type         string            `json:"type"`
	ExternalName string            `json:"externalName"`
	Ports        []struct {
		Name     string `json:"name"`
		Port     string `json:"port"`
		Target   string `json:"targetPort"`
		Protocol string `json:"protocol"`
	} `json:"ports"`
}

type TopologySelector struct {
	Labels      map[string]string `json:"matchLabels"`
	Expressions []struct {
		Key      string   `json:"key"`
		Operator string   `json:"operator"`
		Values   []string `json:"values"`
	} `json:"matchExpressions"`
}

func (selector TopologySelector) Matches(labels map[string]string) bool {
	if len(selector.Labels) == 0 && len(selector.Expressions) == 0 {
		return true
	}
	if labels == nil {
		return false
	}
	for key, value := range selector.Labels {
		if actual, ok := labels[key]; !ok || actual != value {
			return false
		}
	}
	for _, expression := range selector.Expressions {
		value, present := labels[expression.Key]
		included := false
		for _, expected := range expression.Values {
			if expected == value {
				included = true
			}
		}
		switch expression.Operator {
		case "In":
			if !present || !included {
				return false
			}
		case "NotIn":
			if present && included {
				return false
			}
		case "Exists":
			if !present {
				return false
			}
		case "DoesNotExist":
			if present {
				return false
			}
		default:
			return false
		}
	}
	return true
}

type TopologyPeer struct {
	Namespace *TopologySelector `json:"namespaceSelector"`
	Pod       *TopologySelector `json:"podSelector"`
	IPBlock   *struct {
		CIDR   string   `json:"cidr"`
		Except []string `json:"except"`
	} `json:"ipBlock"`
}

type TopologyRule struct {
	From  []TopologyPeer `json:"from"`
	To    []TopologyPeer `json:"to"`
	Ports []struct {
		Port     any    `json:"port"`
		End      any    `json:"endPort"`
		Protocol string `json:"protocol"`
	} `json:"ports"`
}

type TopologyPolicy struct {
	Name     string           `json:"name"`
	NS       string           `json:"ns"`
	Selector TopologySelector `json:"selector"`
	Types    []string         `json:"types"`
	Ingress  []TopologyRule   `json:"ingress"`
	Egress   []TopologyRule   `json:"egress"`
}

type TopologyInput struct {
	Edges              []TopologyEdge               `json:"edges"`
	Workloads          []TopologyWorkload           `json:"workloads"`
	Services           []TopologyService            `json:"services"`
	Policies           []TopologyPolicy             `json:"policies"`
	Namespaces         map[string]map[string]string `json:"namespaces"`
	ExternalHosts      []string                     `json:"external_hosts"`
	SeparatelyOperated []struct {
		NS     string            `json:"ns"`
		Name   string            `json:"name"`
		Port   string            `json:"port"`
		Labels map[string]string `json:"labels"`
	} `json:"separately_operated"`
}

func topologyDescription(workload TopologyWorkload) string {
	return workload.Kind + " " + workload.NS + "/" + workload.Name
}

func (topology TopologyInput) peerMatches(peer TopologyPeer, policyNS, peerNS string, labels map[string]string) bool {
	if peer.IPBlock != nil {
		return (peer.IPBlock.CIDR == "0.0.0.0/0" || peer.IPBlock.CIDR == "::/0") && len(peer.IPBlock.Except) == 0
	}
	if peer.Namespace == nil && peer.Pod == nil {
		return false
	}
	nsOK := peerNS == policyNS
	if peer.Namespace != nil {
		nsLabels := map[string]string{"kubernetes.io/metadata.name": peerNS}
		for key, value := range topology.Namespaces[peerNS] {
			nsLabels[key] = value
		}
		nsOK = peer.Namespace.Matches(nsLabels)
	}
	return nsOK && (peer.Pod == nil || peer.Pod.Matches(labels))
}

func topologyPortMatches(rule TopologyRule, protocol, port string, names map[string][2]string) bool {
	if len(rule.Ports) == 0 {
		return true
	}
	for _, entry := range rule.Ports {
		expected := entry.Protocol
		if expected == "" {
			expected = "TCP"
		}
		if expected != protocol {
			continue
		}
		if entry.Port == nil {
			return true
		}
		start := fmt.Sprint(entry.Port)
		if start == port {
			return true
		}
		if named, exists := names[start]; exists && named[0] == port {
			return true
		}
		first, err := strconv.Atoi(start)
		if err != nil {
			continue
		}
		last, err := strconv.Atoi(fmt.Sprint(entry.End))
		if err != nil {
			continue
		}
		current, err := strconv.Atoi(port)
		if err == nil && first <= current && current <= last {
			return true
		}
	}
	return false
}

func (topology TopologyInput) policiesFor(workload TopologyWorkload, direction string) []TopologyPolicy {
	var policies []TopologyPolicy
	for _, policy := range topology.Policies {
		if policy.NS != workload.NS || !policy.Selector.Matches(workload.Labels) {
			continue
		}
		for _, kind := range policy.Types {
			if kind == direction {
				policies = append(policies, policy)
				break
			}
		}
	}
	return policies
}

func (topology TopologyInput) ingress(callee TopologyWorkload, port, protocol string, caller TopologyWorkload) (bool, string) {
	policies := topology.policiesFor(callee, "Ingress")
	if len(policies) == 0 {
		return true, "no ingress NetworkPolicy selects the callee (open)"
	}
	names := []string{}
	for _, policy := range policies {
		names = append(names, policy.Name)
		for _, rule := range policy.Ingress {
			peerOK := len(rule.From) == 0
			for _, peer := range rule.From {
				peerOK = peerOK || topology.peerMatches(peer, policy.NS, caller.NS, caller.Labels)
			}
			if peerOK && topologyPortMatches(rule, protocol, port, callee.Ports) {
				return true, "ingress admitted by " + policy.Name
			}
		}
	}
	return false, fmt.Sprintf("ingress NetworkPolicy %s does not admit %s on %s/%s", strings.Join(names, ", "), topologyDescription(caller), port, protocol)
}

func (topology TopologyInput) egress(caller TopologyWorkload, calleeNS string, labels map[string]string, port, protocol string, portNames map[string][2]string) (bool, string) {
	if caller.Labels == nil {
		return true, "caller egress is outside these manifests"
	}
	policies := topology.policiesFor(caller, "Egress")
	if len(policies) == 0 {
		return true, "no egress NetworkPolicy selects the caller (open)"
	}
	admitted, dns := "", ""
	names := []string{}
	for _, policy := range policies {
		names = append(names, policy.Name)
		for _, rule := range policy.Egress {
			peerOK, dnsOK := len(rule.To) == 0, len(rule.To) == 0
			for _, peer := range rule.To {
				peerOK = peerOK || topology.peerMatches(peer, policy.NS, calleeNS, labels)
				dnsOK = dnsOK || topology.peerMatches(peer, policy.NS, "kube-system", map[string]string{"k8s-app": "kube-dns"})
			}
			if peerOK && topologyPortMatches(rule, protocol, port, portNames) && admitted == "" {
				admitted = policy.Name
			}
			if dnsOK && topologyPortMatches(rule, "UDP", "53", nil) && dns == "" {
				dns = policy.Name
			}
		}
	}
	if admitted == "" {
		return false, fmt.Sprintf("egress NetworkPolicy %s does not admit %s/%s to the callee", strings.Join(names, ", "), port, protocol)
	}
	if dns == "" {
		return false, fmt.Sprintf("egress NetworkPolicy %s does not admit DNS (UDP 53 to kube-system/kube-dns) so the hostname cannot resolve", strings.Join(names, ", "))
	}
	return true, fmt.Sprintf("egress admitted by %s (DNS via %s)", admitted, dns)
}

func CheckTopology(topology TopologyInput) [][3]string {
	results := make([][3]string, 0, len(topology.Edges))
	services := map[[2]string]TopologyService{}
	for _, service := range topology.Services {
		services[[2]string{service.NS, service.Name}] = service
	}
	for _, edge := range topology.Edges {
		label := fmt.Sprintf("%s -> %s:%s [%s]", topologyDescription(edge.Caller), edge.Host, edge.Port, edge.Source)
		ns, name, kind, err := ResolveServiceHost(edge.Host, edge.Caller.NS, topology.ExternalHosts)
		if err != nil {
			results = append(results, [3]string{"FAIL", label, err.Error()})
			continue
		}
		if kind == "offcluster" || kind == "loopback" {
			status, detail := "note", "explicitly declared external or loopback host; not checked against Service policies"
			if edge.Listed {
				status = "offcluster"
			}
			results = append(results, [3]string{status, label, detail})
			continue
		}
		service, exists := services[[2]string{ns, name}]
		if !exists {
			handled := false
			for _, external := range topology.SeparatelyOperated {
				if external.NS != ns || external.Name != name {
					continue
				}
				handled = true
				if edge.Port != external.Port {
					results = append(results, [3]string{"FAIL", label, fmt.Sprintf("separately operated service %s/%s is declared on port %s, not %s", ns, name, external.Port, edge.Port)})
					break
				}
				ok, detail := topology.egress(edge.Caller, ns, external.Labels, external.Port, "TCP", nil)
				if !ok {
					results = append(results, [3]string{"FAIL", label, detail})
				} else {
					results = append(results, [3]string{"external", label, "separately operated; " + detail + "; callee policy is not in this repository"})
				}
				break
			}
			if !handled {
				results = append(results, [3]string{"FAIL", label, fmt.Sprintf("no Service %s/%s is declared in the manifests and it is not a declared separately operated service", ns, name)})
			}
			continue
		}
		aliases := map[[2]string]bool{}
		aliasFailed := false
		for service.Type == "ExternalName" {
			key := [2]string{ns, name}
			if aliases[key] {
				results = append(results, [3]string{"FAIL", label, fmt.Sprintf("ExternalName cycle at %s/%s", ns, name)})
				aliasFailed = true
				break
			}
			aliases[key] = true
			targetNS, targetName, targetKind, err := ResolveServiceHost(service.ExternalName, ns, topology.ExternalHosts)
			if err != nil || targetKind != "service" {
				results = append(results, [3]string{"FAIL", label, fmt.Sprintf("ExternalName %s/%s target %s has no declared in-cluster Service", ns, name, service.ExternalName)})
				aliasFailed = true
				break
			}
			ns, name = targetNS, targetName
			service, exists = services[[2]string{ns, name}]
			if !exists {
				results = append(results, [3]string{"FAIL", label, fmt.Sprintf("ExternalName target Service %s/%s is not declared", ns, name)})
				aliasFailed = true
				break
			}
		}
		if aliasFailed {
			continue
		}
		portIndex := -1
		for i, port := range service.Ports {
			if edge.Port == port.Port || edge.Port == port.Name {
				portIndex = i
				break
			}
		}
		if portIndex == -1 {
			exposed := []string{}
			for _, port := range service.Ports {
				name := port.Name
				if name == "" {
					name = "-"
				}
				exposed = append(exposed, port.Port+"("+name+")")
			}
			results = append(results, [3]string{"FAIL", label, fmt.Sprintf("Service %s/%s exposes %s, not %s", ns, name, strings.Join(exposed, ", "), edge.Port)})
			continue
		}
		port := service.Ports[portIndex]
		selected := []TopologyWorkload{}
		for _, workload := range topology.Workloads {
			if workload.NS != ns || len(service.Selector) == 0 {
				continue
			}
			matches := true
			for key, value := range service.Selector {
				if workload.Labels[key] != value {
					matches = false
				}
			}
			if matches {
				selected = append(selected, workload)
			}
		}
		if len(selected) == 0 {
			results = append(results, [3]string{"FAIL", label, fmt.Sprintf("Service %s/%s selects no workload declared in the manifests", ns, name)})
			continue
		}
		failed := false
		details := []string{}
		for _, callee := range selected {
			podPort, protocol := port.Target, port.Protocol
			if _, err := strconv.ParseUint(podPort, 10, 16); err != nil {
				named, exists := callee.Ports[podPort]
				if !exists {
					results = append(results, [3]string{"FAIL", label, fmt.Sprintf("Service %s/%s targetPort %s is not a container port of %s", ns, name, podPort, topologyDescription(callee))})
					failed = true
					continue
				}
				podPort, protocol = named[0], named[1]
			}
			ok, detail := topology.ingress(callee, podPort, protocol, edge.Caller)
			if !ok {
				results = append(results, [3]string{"FAIL", label, detail})
				failed = true
				continue
			}
			details = append(details, fmt.Sprintf("pod %s:%s/%s %s", topologyDescription(callee), podPort, protocol, detail))
			ok, detail = topology.egress(edge.Caller, ns, callee.Labels, podPort, protocol, callee.Ports)
			if !ok {
				results = append(results, [3]string{"FAIL", label, detail})
				failed = true
				continue
			}
			details = append(details, detail)
		}
		if !failed {
			results = append(results, [3]string{"ok", label, strings.Join(details, "; ")})
		}
	}
	return results
}
