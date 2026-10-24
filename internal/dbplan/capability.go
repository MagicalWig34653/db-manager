// SPDX-License-Identifier: AGPL-3.0-only
package dbplan

import "sort"

// Support is how a driver supports an action.
type Support string

const (
	Supported   Support = "supported"
	Unsupported Support = "unsupported"
	Limited     Support = "limited"
)

// Capability describes one action for one driver. Reason is required for
// Unsupported and Limited so the UI can explain instead of hiding the action.
type Capability struct {
	Support Support `json:"support"`
	Reason  string  `json:"reason,omitempty"`
}

// Valid reports whether the capability is well formed.
func (c Capability) Valid() bool {
	switch c.Support {
	case Supported:
		return true
	case Unsupported, Limited:
		return c.Reason != ""
	}
	return false
}

// Capabilities maps driver -> action -> capability.
type Capabilities map[string]map[string]Capability

// Set records a capability; it panics on a malformed value because capability
// tables are static program data and an invalid one is a programming error.
func (c Capabilities) Set(driver, action string, capability Capability) {
	if driver == "" || action == "" || !capability.Valid() {
		panic("dbplan: invalid capability for " + driver + "/" + action)
	}
	if c[driver] == nil {
		c[driver] = map[string]Capability{}
	}
	c[driver][action] = capability
}

// Get returns the capability; an unknown driver or action is Unsupported.
func (c Capabilities) Get(driver, action string) Capability {
	if found, ok := c[driver][action]; ok {
		return found
	}
	return Capability{Support: Unsupported, Reason: "Nicht unterstützt."}
}

// Allows reports whether the action may be attempted (supported or limited).
func (c Capabilities) Allows(driver, action string) bool {
	return c.Get(driver, action).Support != Unsupported
}

// Drivers returns the known drivers in stable order.
func (c Capabilities) Drivers() []string {
	out := make([]string, 0, len(c))
	for driver := range c {
		out = append(out, driver)
	}
	sort.Strings(out)
	return out
}
