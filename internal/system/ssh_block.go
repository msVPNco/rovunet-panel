package system

import "strings"

// Old installations did not delimit the end of their Match block. Stop at
// the next Match, preserving later administrator rules and custom directives.
func replaceOwnedSSHBlock(content string, start int) string {
	lines := strings.Split(content[start:], "\n")
	end := len(lines)
	seen := false
	owned := map[string]bool{"passwordauthentication": true, "authenticationmethods": true, "allowtcpforwarding": true, "allowstreamlocalforwarding": true, "gatewayports": true, "permittunnel": true, "permittty": true, "permituserrc": true, "x11forwarding": true, "allowagentforwarding": true, "forcecommand": true, "clientaliveinterval": true, "clientalivecountmax": true}
	var extras []string
	for i, line := range lines {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		if strings.EqualFold(fields[0], "Match") {
			if seen {
				end = i
				break
			}
			seen = true
			if len(fields) != 3 || !strings.EqualFold(fields[1], "Group") || fields[2] != "rovunet-ssh" {
				return content
			}
			continue
		}
		if line == "# Rovunet SSH tunnel users" {
			continue
		}
		if !owned[strings.ToLower(fields[0])] {
			extras = append(extras, line)
		}
	}
	if !seen {
		return content
	}
	next := strings.TrimRight(content[:start], "\n") + sshMatchBlock
	if len(extras) > 0 {
		next += strings.Join(extras, "\n") + "\n"
	}
	if end < len(lines) {
		next += strings.Join(lines[end:], "\n")
	}
	return next
}
