package dsync

import (
	"fmt"
	"regexp"
	"strings"
)

type (
	Resource struct {
		Namespace string // optional
		Scope     string // optional
		Name      string
	}
)

// NamePattern is enforced kubernetes on lease names.
// https://regex101.com/r/9zbjQ0/1
// example: kebab-case-with-0-9.seperated-by-dots
//
// Matches a domain-like string that:
// - Starts and ends with an alphanumeric character (a-z0-9).
// - Can have multiple segments separated by dots (.).
// - Each segment must start and end with an alphanumeric character and may contain hyphens in between.
// - Supports optional subdomains or multiple levels.
// Examples that match:
// - example
// - example.com
// - sub.domain.com
// - a1-b2.c3-d4
// Examples that don’t match:
// - .example (starts with a dot)
// - example. (ends with a dot)
// - ex..ample (consecutive dots)
const NamePattern = `^[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*$`

func NewResource(name string) Resource { return Resource{Name: name} }
func NewScopedResource(ns, scope, name string) Resource {
	return Resource{
		Namespace: ns,
		Scope:     scope,
		Name:      name,
	}
}
func ToResource(s string) Resource {
	var (
		in = strings.SplitN(s, "/", 3)
	)
	if len(in) == 3 {
		return Resource{
			Namespace: in[0],
			Scope:     in[1],
			Name:      in[2],
		}
	}
	if len(in) == 2 {
		return Resource{
			Scope: in[0],
			Name:  in[1],
		}
	}
	return NewResource(s)
}
func (r Resource) String() string {
	var (
		hasNS = r.Namespace != ""
		hasSC = r.Scope != ""
	)
	if hasNS && hasSC {
		return fmt.Sprintf("%s/%s/%s", r.Namespace, r.Scope, r.Name)
	} else if hasSC {
		return fmt.Sprintf("%s/%s", r.Scope, r.Name)
	}
	return r.Name
}
func (r Resource) Equal(r2 Resource) bool {
	return r.String() == r2.String()
}
func (r Resource) Validate() error {
	rx, err := regexp.Compile(NamePattern)
	if err != nil {
		return fmt.Errorf("regex.Compile dsync.NamePattern error: %w", err)
	}
	for _, s := range []string{r.ElectionName()} {
		if ok := rx.MatchString(s); !ok {
			return ErrBadResourceName
		}
	}
	return nil
}
func (r Resource) ElectionName() string { return r.getName("election") }
func (r Resource) LockName() string     { return r.getName("lock") }
func (r Resource) getName(kind string) string {
	scope, task := r.Scope, r.Name
	if scope == "" {
		scope = DefaultScope
	}
	if task == "" {
		task = MasterTask
	}
	return fmt.Sprintf("%s.%s.%s", scope, kind, task)
}
