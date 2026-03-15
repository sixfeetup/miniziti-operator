/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package policy

import (
	"strings"

	zitiv1alpha1 "example.com/miniziti-operator/api/v1alpha1"
)

// CompileSelector renders a selector into the OpenZiti role expressions used by the backend API.
func CompileSelector(selector zitiv1alpha1.SelectorSpec) []string {
	compiled := make([]string, 0, len(selector.MatchNames)+len(selector.MatchRoleAttributes))
	seen := make(map[string]struct{}, len(selector.MatchNames)+len(selector.MatchRoleAttributes))

	for _, matchName := range selector.MatchNames {
		expression := "@" + strings.TrimSpace(matchName)
		if expression == "@" {
			continue
		}
		if _, ok := seen[expression]; ok {
			continue
		}
		seen[expression] = struct{}{}
		compiled = append(compiled, expression)
	}

	for _, attribute := range selector.MatchRoleAttributes {
		expression := "#" + strings.TrimSpace(attribute)
		if expression == "#" {
			continue
		}
		if _, ok := seen[expression]; ok {
			continue
		}
		seen[expression] = struct{}{}
		compiled = append(compiled, expression)
	}

	return compiled
}
