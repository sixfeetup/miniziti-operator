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

package service

import (
	zitiv1alpha1 "example.com/miniziti-operator/api/v1alpha1"
	openziti "example.com/miniziti-operator/internal/openziti/client"
)

// DesiredService captures the normalized operator view of a ZitiService spec.
type DesiredService struct {
	Name           string
	RoleAttributes []string
	ConfigIDs      []string
	Intercept      openziti.ServiceConfig
	Host           openziti.ServiceConfig
}

// FromResource maps a ZitiService resource into service and config payloads.
func FromResource(resource *zitiv1alpha1.ZitiService) DesiredService {
	return DesiredService{
		Name:           resource.Spec.Name,
		RoleAttributes: append([]string(nil), resource.Spec.RoleAttributes...),
		Intercept: openziti.ServiceConfig{
			Name: resource.Spec.Name + "-intercept",
			Type: "intercept.v1",
			Payload: map[string]any{
				"protocols":  append([]string(nil), resource.Spec.Configs.Intercept.Protocols...),
				"addresses":  append([]string(nil), resource.Spec.Configs.Intercept.Addresses...),
				"portRanges": toPortRanges(resource.Spec.Configs.Intercept.PortRanges),
			},
		},
		Host: openziti.ServiceConfig{
			Name: resource.Spec.Name + "-host",
			Type: "host.v1",
			Payload: map[string]any{
				"protocol": resource.Spec.Configs.Host.Protocol,
				"address":  resource.Spec.Configs.Host.Address,
				"port":     resource.Spec.Configs.Host.Port,
			},
		},
	}
}

func toPortRanges(ranges []zitiv1alpha1.PortRange) []map[string]int32 {
	result := make([]map[string]int32, 0, len(ranges))
	for _, portRange := range ranges {
		result = append(result, map[string]int32{
			"low":  portRange.Low,
			"high": portRange.High,
		})
	}
	return result
}
